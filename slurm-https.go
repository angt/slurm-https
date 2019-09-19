package main

/*
#cgo pkg-config: slurm

#include <stdio.h>
#include <stdlib.h>
#include <signal.h>

#include "slurm/slurm.h"
#include "slurm/slurm_errno.h"

typedef char* chars;

#define SLUW_LIST(T,S) \
T *sluw_alloc_##T (int s) { T *r = (T *)calloc(s+1, sizeof(T)); if (r) r[s]=S; return r;} \
void sluw_set_##T (T *l, T v, int p) { l[p]=v; } \
size_t sluw_len_##T (T *l) { size_t i=0; if (l) while (l[i]!=S) i++; return i;}

SLUW_LIST(uint32_t,0)
SLUW_LIST(int32_t,-1)
SLUW_LIST(chars,NULL)
*/
import (
	"C"
)

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"unsafe"
)

func slurm_error(w http.ResponseWriter, r *http.Request) {
	errno := C.slurm_get_errno()
	errno_str := "SLURM-" + strconv.Itoa(int(errno)) + " " + C.GoString(C.slurm_strerror(errno))
	log.Println("from:", r.RemoteAddr, "request:", r.RequestURI, errno_str)
	http.Error(w, errno_str, 500)
}

func sluw_get_name(s string) string {
	if len(s) < 2 {
		return strings.ToUpper(s)
	}

	str := strings.Split(s, "_")

	for i, v := range str {
		tmp := strings.SplitN(v, "", 2)
		tmp[0] = strings.ToUpper(tmp[0])
		str[i] = strings.Join(tmp, "")
	}

	return strings.Join(str, "")
}

type object struct {
	Type   string
	Offset unsafe.Pointer
}

type object_map map[string]object

func (t object_map) Add(data interface{}) {
	val := reflect.ValueOf(data)

	if val.Kind() != reflect.Ptr {
		return
	}

	val = val.Elem()

	if !val.CanAddr() {
		return
	}

	ptr := val.Addr().Pointer()

	for i := 0; i < val.NumField(); i++ {
		name := val.Type().Field(i).Name
		offset := val.Type().Field(i).Offset
		field := val.Field(i)
		if name == "_" {
			continue
		}
		t[sluw_get_name(name)] = object{
			field.Type().String(),
			unsafe.Pointer(ptr + offset),
		}
	}
}

func (t object_map) Run(w http.ResponseWriter, r *http.Request, fn func()) {
	var req map[string]*json.RawMessage
	err := json.NewDecoder(r.Body).Decode(&req)

	if err != nil {
		http.Error(w, "Bad request", 400)
		return
	}

	for key, value := range req {
		dst, ok := t[key]

		if !ok {
			http.Error(w, "Unknown key: "+key, 400)
			return
		}

		var err error

		switch dst.Type {
		case "*main._Ctype_char":
			var s string
			err = json.Unmarshal(*value, &s)
			tmp := C.CString(s)
			*(**C.char)(dst.Offset) = tmp
			defer C.free(unsafe.Pointer(tmp))
		case "main._Ctype_uint32_t":
			var i uint32
			err = json.Unmarshal(*value, &i)
			*(*C.uint32_t)(dst.Offset) = C.uint32_t(i)
		case "main._Ctype_uint16_t":
			var i uint16
			err = json.Unmarshal(*value, &i)
			*(*C.uint16_t)(dst.Offset) = C.uint16_t(i)
		case "main._Ctype_uint8_t":
			var i uint8
			err = json.Unmarshal(*value, &i)
			*(*C.uint8_t)(dst.Offset) = C.uint8_t(i)
		case "main._Ctype_int32_t":
			var i int32
			err = json.Unmarshal(*value, &i)
			*(*C.int32_t)(dst.Offset) = C.int32_t(i)
		case "main._Ctype_int16_t":
			var i int16
			err = json.Unmarshal(*value, &i)
			*(*C.int16_t)(dst.Offset) = C.int16_t(i)
		case "main._Ctype_int8_t":
			var i int8
			err = json.Unmarshal(*value, &i)
			*(*C.int8_t)(dst.Offset) = C.int8_t(i)
		case "main._Ctype_time_t":
			var i uint64
			err = json.Unmarshal(*value, &i)
			*(*C.time_t)(dst.Offset) = C.time_t(i)
		case "*main._Ctype_uint32_t":
			var ai []uint32
			err = json.Unmarshal(*value, &ai)
			tmp := C.sluw_alloc_uint32_t(C.int(len(ai)))
			defer C.free(unsafe.Pointer(tmp))
			for i := 0; i < len(ai); i++ {
				C.sluw_set_uint32_t(tmp, C.uint32_t(ai[i]), C.int(i))
			}
			*(**C.uint32_t)(dst.Offset) = tmp
		case "*main._Ctype_int32_t":
			var ai []int32
			err = json.Unmarshal(*value, &ai)
			tmp := C.sluw_alloc_int32_t(C.int(len(ai)))
			defer C.free(unsafe.Pointer(tmp))
			for i := 0; i < len(ai); i++ {
				C.sluw_set_int32_t(tmp, C.int32_t(ai[i]), C.int(i))
			}
			*(**C.int32_t)(dst.Offset) = tmp
		case "**main._Ctype_char":
			var as []string
			err = json.Unmarshal(*value, &as)
			tmp := C.sluw_alloc_chars(C.int(len(as)))
			defer C.free(unsafe.Pointer(tmp))
			for i := 0; i < len(as); i++ {
				tmp2 := C.CString(as[i])
				defer C.free(unsafe.Pointer(tmp2))
				C.sluw_set_chars(tmp, tmp2, C.int(i))
			}
			*(***C.char)(dst.Offset) = (**C.char)(tmp)
		default:
			log.Println(key, reflect.TypeOf(dst), "not supported")
		}

		if err != nil {
			http.Error(w, "Bad value for key: "+key, 400)
			return
		}
	}

	fn()
}

type table map[string]interface{}

func get_res(data interface{}) *table {
	ret := make(table)
	val := reflect.ValueOf(data).Elem()
	for i := 0; i < val.NumField(); i++ {
		f := val.Type().Field(i)
		if f.Name == "_" {
			continue
		}
		name := sluw_get_name(f.Name)
		v := val.Field(i)
		switch f.Type.String() {
		case "main._Ctype_uint8_t",
			"main._Ctype_uint16_t",
			"main._Ctype_uint32_t",
			"main._Ctype_uint64_t":
			ret[name] = uint(v.Uint())
		case "main._Ctype_int8_t",
			"main._Ctype_int16_t",
			"main._Ctype_int32_t",
			"main._Ctype_int64_t",
			"main._Ctype_time_t": // why not..
			ret[name] = int(v.Int())
		case "*main._Ctype_uint32_t":
			if v.Pointer() == 0 {
				ret[name] = make([]uint, 0)
				break
			}
			data := unsafe.Pointer(v.Pointer())
			count := int(C.sluw_len_uint32_t((*C.uint32_t)(data)))
			array := make([]uint, count)
			carray := *(*[]C.uint32_t)(unsafe.Pointer(&reflect.SliceHeader{
				Data: uintptr(data),
				Len:  count,
				Cap:  count,
			}))
			for k := 0; k < count; k++ {
				array[k] = uint(carray[k])
			}
			ret[name] = array
		case "*main._Ctype_int32_t":
			if v.Pointer() == 0 {
				ret[name] = make([]int, 0)
				break
			}
			data := unsafe.Pointer(v.Pointer())
			count := int(C.sluw_len_int32_t((*C.int32_t)(data)))
			array := make([]int, count)
			carray := *(*[]C.int32_t)(unsafe.Pointer(&reflect.SliceHeader{
				Data: uintptr(data),
				Len:  count,
				Cap:  count,
			}))
			for k := 0; k < count; k++ {
				array[k] = int(carray[k])
			}
			ret[name] = array
		case "*main._Ctype_char":
			if v.Pointer() == 0 {
				ret[name] = nil
				break
			}
			ret[name] = C.GoString((*C.char)(unsafe.Pointer(v.Pointer())))
		default:
			log.Println(name, f.Type, "not supported")
		}
	}

	return &ret
}

func submit_batch_job(w http.ResponseWriter, r *http.Request) {
	var slreq C.job_desc_msg_t
	C.slurm_init_job_desc_msg(&slreq)

	obj := make(object_map)
	obj.Add(&slreq)

	obj.Run(w, r, func() {
		var slres *C.submit_response_msg_t

		ret := C.slurm_submit_batch_job(&slreq, &slres)

		if ret != 0 {
			slurm_error(w, r)
			return
		}

		res := get_res(slres)
		C.slurm_free_submit_response_response_msg(slres)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&res)
	})
}

func notify_job(w http.ResponseWriter, r *http.Request) {
	opt := struct {
		job_id  C.uint32_t
		message *C.char
	}{}

	obj := make(object_map)
	obj.Add(&opt)

	obj.Run(w, r, func() {
		ret := C.slurm_notify_job(opt.job_id, opt.message)

		if ret != 0 {
			slurm_error(w, r)
			return
		}
	})
}

func update_job(w http.ResponseWriter, r *http.Request) {
	var slreq C.job_desc_msg_t
	C.slurm_init_job_desc_msg(&slreq)

	obj := make(object_map)
	obj.Add(&slreq)

	obj.Run(w, r, func() {
		ret := C.slurm_update_job(&slreq)

		if ret != 0 {
			slurm_error(w, r)
			return
		}
	})
}

func load_jobs(w http.ResponseWriter, r *http.Request) {
	opt := struct {
		update_time C.time_t
		show_flags  C.uint16_t
	}{}

	obj := make(object_map)
	obj.Add(&opt)

	obj.Run(w, r, func() {
		var slres *C.job_info_msg_t

		ret := C.slurm_load_jobs(opt.update_time, &slres, opt.show_flags)

		if ret != 0 {
			slurm_error(w, r)
			return
		}

		data := unsafe.Pointer(slres.job_array)
		count := int(slres.record_count)
		carray := *(*[]C.job_info_t)(unsafe.Pointer(&reflect.SliceHeader{
			Data: uintptr(data),
			Len:  count,
			Cap:  count,
		}))

		res := get_res(slres)

		array := make([]*table, count)
		for i := 0; i < count; i++ {
			array[i] = get_res(&carray[i])
		}

		(*res)["JobArray"] = array

		C.slurm_free_job_info_msg(slres)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&res)
	})
}

func load_node(w http.ResponseWriter, r *http.Request) {
	opt := struct {
		update_time C.time_t
		show_flags  C.uint16_t
	}{}

	obj := make(object_map)
	obj.Add(&opt)

	obj.Run(w, r, func() {
		var slres *C.node_info_msg_t

		ret := C.slurm_load_node(opt.update_time, &slres, opt.show_flags)

		if ret != 0 {
			slurm_error(w, r)
			return
		}

		data := unsafe.Pointer(slres.node_array)
		count := int(slres.record_count)
		carray := *(*[]C.node_info_t)(unsafe.Pointer(&reflect.SliceHeader{
			Data: uintptr(data),
			Len:  count,
			Cap:  count,
		}))

		res := get_res(slres)
		array := make([]*table, count)

		for i := 0; i < count; i++ {
			array[i] = get_res(&carray[i])
		}

		(*res)["NodeArray"] = array

		C.slurm_free_node_info_msg(slres)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&res)
	})
}

func update_node(w http.ResponseWriter, r *http.Request) {
	var slreq C.update_node_msg_t
	C.slurm_init_update_node_msg(&slreq)

	obj := make(object_map)
	obj.Add(&slreq)

	obj.Run(w, r, func() {
		ret := C.slurm_update_node(&slreq)

		if ret != 0 {
			slurm_error(w, r)
			return
		}
	})
}

func signal_job(w http.ResponseWriter, r *http.Request) {
	opt := struct {
		job_id C.uint32_t
		signal C.uint16_t
	}{}

	obj := make(object_map)
	obj.Add(&opt)

	obj.Run(w, r, func() {
		ret := C.slurm_signal_job(opt.job_id, opt.signal)

		if ret != 0 {
			slurm_error(w, r)
			return
		}
	})
}

func signal_job_step(w http.ResponseWriter, r *http.Request) {
	opt := struct {
		job_id  C.uint32_t
		step_id C.uint32_t
		signal  C.uint32_t
	}{}

	obj := make(object_map)
	obj.Add(&opt)

	obj.Run(w, r, func() {
		ret := C.slurm_signal_job_step(opt.job_id, opt.step_id, opt.signal)

		if ret != 0 {
			slurm_error(w, r)
			return
		}
	})
}

func kill_job(w http.ResponseWriter, r *http.Request) {
	opt := struct {
		job_id     C.uint32_t
		signal     C.uint16_t
		batch_flag C.uint16_t
	}{}

	obj := make(object_map)
	obj.Add(&opt)

	obj.Run(w, r, func() {
		ret := C.slurm_kill_job(opt.job_id, opt.signal, opt.batch_flag)

		if ret != 0 {
			slurm_error(w, r)
			return
		}
	})
}

func kill_job_step(w http.ResponseWriter, r *http.Request) {
	opt := struct {
		job_id  C.uint32_t
		step_id C.uint32_t
		signal  C.uint16_t
	}{}

	obj := make(object_map)
	obj.Add(&opt)

	obj.Run(w, r, func() {
		ret := C.slurm_kill_job_step(opt.job_id, opt.step_id, opt.signal)

		if ret != 0 {
			slurm_error(w, r)
			return
		}
	})
}

func complete_job(w http.ResponseWriter, r *http.Request) {
	opt := struct {
		job_id          C.uint32_t
		job_return_code C.uint32_t
	}{}

	obj := make(object_map)
	obj.Add(&opt)

	obj.Run(w, r, func() {
		ret := C.slurm_complete_job(opt.job_id, opt.job_return_code)

		if ret != 0 {
			slurm_error(w, r)
			return
		}
	})
}

func terminate_job_step(w http.ResponseWriter, r *http.Request) {
	opt := struct {
		job_id  C.uint32_t
		step_id C.uint32_t
	}{}

	obj := make(object_map)
	obj.Add(&opt)

	obj.Run(w, r, func() {
		ret := C.slurm_terminate_job_step(opt.job_id, opt.step_id)

		if ret != 0 {
			slurm_error(w, r)
			return
		}
	})
}

func suspend_job(w http.ResponseWriter, r *http.Request) {
	opt := struct {
		job_id C.uint32_t
	}{}

	obj := make(object_map)
	obj.Add(&opt)

	obj.Run(w, r, func() {
		ret := C.slurm_suspend(opt.job_id)

		if ret != 0 {
			slurm_error(w, r)
			return
		}
	})
}

func resume_job(w http.ResponseWriter, r *http.Request) {
	opt := struct {
		job_id C.uint32_t
	}{}

	obj := make(object_map)
	obj.Add(&opt)

	obj.Run(w, r, func() {
		ret := C.slurm_resume(opt.job_id)

		if ret != 0 {
			slurm_error(w, r)
			return
		}
	})
}

func requeue_job(w http.ResponseWriter, r *http.Request) {
	opt := struct {
		job_id C.uint32_t
		state  C.uint32_t
	}{}

	obj := make(object_map)
	obj.Add(&opt)

	obj.Run(w, r, func() {
		ret := C.slurm_requeue(opt.job_id, opt.state)

		if ret != 0 {
			slurm_error(w, r)
			return
		}
	})
}

func load_licenses(w http.ResponseWriter, r *http.Request) {
	opt := struct {
		update_time C.time_t
		show_flags  C.uint16_t
	}{}

	obj := make(object_map)
	obj.Add(&opt)

	obj.Run(w, r, func() {
		var slres *C.license_info_msg_t

		ret := C.slurm_load_licenses(opt.update_time, &slres, opt.show_flags)

		if ret != 0 {
			slurm_error(w, r)
			return
		}

		data := unsafe.Pointer(slres.lic_array)
		count := int(slres.num_lic)
		carray := *(*[]C.slurm_license_info_t)(unsafe.Pointer(&reflect.SliceHeader{
			Data: uintptr(data),
			Len:  count,
			Cap:  count,
		}))

		res := get_res(slres)
		array := make([]*table, count)

		for i := 0; i < count; i++ {
			array[i] = get_res(&carray[i])
		}

		(*res)["LicArray"] = array

		C.slurm_free_license_info_msg(slres)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&res)
	})
}

func load_reservations(w http.ResponseWriter, r *http.Request) {
	opt := struct {
		update_time C.time_t
	}{}

	obj := make(object_map)
	obj.Add(&opt)

	obj.Run(w, r, func() {
		var slres *C.reserve_info_msg_t

		ret := C.slurm_load_reservations(opt.update_time, &slres)

		if ret != 0 {
			slurm_error(w, r)
			return
		}

		data := unsafe.Pointer(slres.reservation_array)
		count := int(slres.record_count)
		carray := *(*[]C.reserve_info_t)(unsafe.Pointer(&reflect.SliceHeader{
			Data: uintptr(data),
			Len:  count,
			Cap:  count,
		}))

		res := get_res(slres)
		array := make([]*table, count)

		for i := 0; i < count; i++ {
			array[i] = get_res(&carray[i])
		}

		(*res)["ReservationArray"] = array

		C.slurm_free_reservation_info_msg(slres)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&res)
	})
}

func delete_reservation(w http.ResponseWriter, r *http.Request) {
	var slreq C.reservation_name_msg_t

	obj := make(object_map)
	obj.Add(&slreq)

	obj.Run(w, r, func() {
		ret := C.slurm_delete_reservation(&slreq)

		if ret != 0 {
			slurm_error(w, r)
			return
		}
	})
}

func create_reservation(w http.ResponseWriter, r *http.Request) {
	var slreq C.resv_desc_msg_t
	C.slurm_init_resv_desc_msg(&slreq)

	obj := make(object_map)
	obj.Add(&slreq)

	obj.Run(w, r, func() {
		ret := C.slurm_create_reservation(&slreq)

		if ret == nil {
			slurm_error(w, r)
			return
		}

		res := get_res(&slreq)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&res)
	})
}

func update_reservation(w http.ResponseWriter, r *http.Request) {
	var slreq C.resv_desc_msg_t
	C.slurm_init_resv_desc_msg(&slreq)

	obj := make(object_map)
	obj.Add(&slreq)

	obj.Run(w, r, func() {
		ret := C.slurm_update_reservation(&slreq)

		if ret != 0 {
			slurm_error(w, r)
			return
		}
	})
}

func get_triggers(w http.ResponseWriter, r *http.Request) {
	var slres *C.trigger_info_msg_t

	ret := C.slurm_get_triggers(&slres)

	if ret != 0 {
		slurm_error(w, r)
		return
	}

	data := unsafe.Pointer(slres.trigger_array)
	count := int(slres.record_count)
	carray := *(*[]C.trigger_info_t)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(data),
		Len:  count,
		Cap:  count,
	}))

	res := get_res(slres)
	array := make([]*table, count)

	for i := 0; i < count; i++ {
		array[i] = get_res(&carray[i])
	}

	(*res)["TriggerArray"] = array

	C.slurm_free_trigger_msg(slres)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&res)
}

func set_trigger(w http.ResponseWriter, r *http.Request) {
	var slreq C.trigger_info_t
	C.slurm_init_trigger_msg(&slreq)

	obj := make(object_map)
	obj.Add(&slreq)

	obj.Run(w, r, func() {
		ret := C.slurm_set_trigger(&slreq)

		if ret != 0 {
			slurm_error(w, r)
			return
		}
	})
}

func clear_trigger(w http.ResponseWriter, r *http.Request) {
	var slreq C.trigger_info_t
	C.slurm_init_trigger_msg(&slreq)

	obj := make(object_map)
	obj.Add(&slreq)

	obj.Run(w, r, func() {
		ret := C.slurm_clear_trigger(&slreq)

		if ret != 0 {
			slurm_error(w, r)
			return
		}
	})
}

func takeover(w http.ResponseWriter, r *http.Request) {
	opt := struct {
		backup_inx C.int
	}{}

	obj := make(object_map)
	obj.Add(&opt)

	obj.Run(w, r, func() {
		ret := C.slurm_takeover(opt.backup_inx)

		if ret != 0 {
			slurm_error(w, r)
			return
		}
	})
}

func shutdown(w http.ResponseWriter, r *http.Request) {
	opt := struct {
		options C.uint16_t
	}{}

	obj := make(object_map)
	obj.Add(&opt)

	obj.Run(w, r, func() {
		ret := C.slurm_shutdown(opt.options)

		if ret != 0 {
			slurm_error(w, r)
			return
		}
	})
}

func reconfigure(w http.ResponseWriter, r *http.Request) {
	ret := C.slurm_reconfigure()

	if ret != 0 {
		slurm_error(w, r)
		return
	}
}

func ping(w http.ResponseWriter, r *http.Request) {
	opt := struct {
		primary C.int
	}{}

	obj := make(object_map)
	obj.Add(&opt)

	obj.Run(w, r, func() {
		ret := C.slurm_ping(opt.primary)

		if ret != 0 {
			slurm_error(w, r)
			return
		}
	})
}

func load_partitions(w http.ResponseWriter, r *http.Request) {
	opt := struct {
		update_time C.time_t
		show_flags  C.uint16_t
	}{}

	obj := make(object_map)
	obj.Add(&opt)

	obj.Run(w, r, func() {
		var slres *C.partition_info_msg_t

		ret := C.slurm_load_partitions(opt.update_time, &slres, opt.show_flags)

		if ret != 0 {
			slurm_error(w, r)
			return
		}

		data := unsafe.Pointer(slres.partition_array)
		count := int(slres.record_count)
		carray := *(*[]C.partition_info_t)(unsafe.Pointer(&reflect.SliceHeader{
			Data: uintptr(data),
			Len:  count,
			Cap:  count,
		}))

		res := get_res(slres)
		array := make([]*table, count)

		for i := 0; i < count; i++ {
			array[i] = get_res(&carray[i])
		}

		(*res)["PartitionArray"] = array

		C.slurm_free_partition_info_msg(slres)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&res)
	})
}

func create_partition(w http.ResponseWriter, r *http.Request) {
	var slreq C.update_part_msg_t
	C.slurm_init_part_desc_msg(&slreq)

	obj := make(object_map)
	obj.Add(&slreq)

	obj.Run(w, r, func() {
		ret := C.slurm_create_partition(&slreq)

		if ret != 0 {
			slurm_error(w, r)
			return
		}
	})
}

func update_partition(w http.ResponseWriter, r *http.Request) {
	var slreq C.update_part_msg_t
	C.slurm_init_part_desc_msg(&slreq)

	obj := make(object_map)
	obj.Add(&slreq)

	obj.Run(w, r, func() {
		ret := C.slurm_update_partition(&slreq)

		if ret != 0 {
			slurm_error(w, r)
			return
		}
	})
}

func delete_partition(w http.ResponseWriter, r *http.Request) {
	var slreq C.delete_part_msg_t

	obj := make(object_map)
	obj.Add(&slreq)

	obj.Run(w, r, func() {
		ret := C.slurm_delete_partition(&slreq)

		if ret != 0 {
			slurm_error(w, r)
			return
		}
	})
}

func load_topo(w http.ResponseWriter, r *http.Request) {
	var slres *C.topo_info_response_msg_t

	ret := C.slurm_load_topo(&slres)

	if ret != 0 {
		slurm_error(w, r)
		return
	}

	data := unsafe.Pointer(slres.topo_array)
	count := int(slres.record_count)
	carray := *(*[]C.topo_info_t)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(data),
		Len:  count,
		Cap:  count,
	}))

	res := get_res(slres)
	array := make([]*table, count)

	for i := 0; i < count; i++ {
		array[i] = get_res(&carray[i])
	}

	(*res)["TopoArray"] = array

	C.slurm_free_topo_info_msg(slres)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&res)
}

func load_frontend(w http.ResponseWriter, r *http.Request) {
	opt := struct {
		update_time C.time_t
	}{}

	obj := make(object_map)
	obj.Add(&opt)

	obj.Run(w, r, func() {
		var slres *C.front_end_info_msg_t

		ret := C.slurm_load_front_end(opt.update_time, &slres)

		if ret != 0 {
			slurm_error(w, r)
			return
		}

		data := unsafe.Pointer(slres.front_end_array)
		count := int(slres.record_count)
		carray := *(*[]C.front_end_info_t)(unsafe.Pointer(&reflect.SliceHeader{
			Data: uintptr(data),
			Len:  count,
			Cap:  count,
		}))

		res := get_res(slres)
		array := make([]*table, count)

		for i := 0; i < count; i++ {
			array[i] = get_res(&carray[i])
		}

		(*res)["FrontEndArray"] = array

		C.slurm_free_front_end_info_msg(slres)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&res)
	})
}

func update_frontend(w http.ResponseWriter, r *http.Request) {
	var slreq C.update_front_end_msg_t
	C.slurm_init_update_front_end_msg(&slreq)

	obj := make(object_map)
	obj.Add(&slreq)

	obj.Run(w, r, func() {
		ret := C.slurm_update_front_end(&slreq)

		if ret != 0 {
			slurm_error(w, r)
			return
		}
	})
}

func lookup_job(w http.ResponseWriter, r *http.Request) {
	opt := struct {
		job_id C.uint32_t
	}{}

	obj := make(object_map)
	obj.Add(&opt)

	obj.Run(w, r, func() {
		var slres *C.resource_allocation_response_msg_t

		ret := C.slurm_allocation_lookup(opt.job_id, &slres)

		if ret != 0 {
			slurm_error(w, r)
			return
		}

		res := get_res(slres)

		C.slurm_free_resource_allocation_response_msg(slres)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&res)
	})
}

func alloc_job(w http.ResponseWriter, r *http.Request) {
	var slreq C.job_desc_msg_t
	C.slurm_init_job_desc_msg(&slreq)

	obj := make(object_map)
	obj.Add(&slreq)

	obj.Run(w, r, func() {
		var slres *C.resource_allocation_response_msg_t

		ret := C.slurm_allocate_resources(&slreq, &slres)

		if ret != 0 {
			slurm_error(w, r)
			return
		}

		res := get_res(slres)

		C.slurm_free_resource_allocation_response_msg(slres)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&res)
	})
}

func load_ctl_conf(w http.ResponseWriter, r *http.Request) {
	opt := struct {
		update_time C.time_t
	}{}

	obj := make(object_map)
	obj.Add(&opt)

	obj.Run(w, r, func() {
		var slres *C.slurm_ctl_conf_t

		ret := C.slurm_load_ctl_conf(opt.update_time, &slres)

		if ret != 0 {
			slurm_error(w, r)
			return
		}

		res := get_res(slres)

		C.slurm_free_ctl_conf(slres)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&res)
	})
}

/*
func tasks_checkpoint(w http.ResponseWriter, r *http.Request) {
    opt := struct {
        job_id C.uint32_t
        step_id C.uint16_t // XXX
        begin_time C.time_t
        image_dir *C.char
        max_wait C.uint16_t
        node_list *C.char
    }{}

    obj := make(object_map)
    obj.Add(&obj)

    obj.Run(w,r, func(){
        ret := C.slurm_checkpoint_tasks(
            opt.job_id,
            opt.step_id,
            opt.begin_time,
            opt.image_dir,
            opt.max_wait,
            opt.node_list,
        )

        if ret != 0 {
            slurm_error(w,r)
            return
        }
    })
}
*/

func main() {
	var (
		addr = flag.String("addr", ":8443", "adresse to serve")
		cert = flag.String("cert", "server.crt", "certificate")
		key  = flag.String("key", "server.key", "certificate key")
		ca   = flag.String("ca", "ca.crt", "ca certificate")
	)

	flag.Parse()

	ca_cert, err := ioutil.ReadFile(*ca)

	if err != nil {
		log.Fatal(err)
	}

	ca_pool := x509.NewCertPool()
	ca_pool.AppendCertsFromPEM(ca_cert)

	server := &http.Server{
		Addr: *addr,
		TLSConfig: &tls.Config{
			ClientAuth: tls.RequireAndVerifyClientCert,
			ClientCAs:  ca_pool,
		},
	}

	// this api is only for test... no comment :)

	http.HandleFunc("/nodes", load_node)
	http.HandleFunc("/node/update", update_node)

	http.HandleFunc("/licenses", load_licenses)

	http.HandleFunc("/conf", load_ctl_conf)

	http.HandleFunc("/jobs", load_jobs)
	http.HandleFunc("/job/alloc", alloc_job)
	http.HandleFunc("/job/submit", submit_batch_job)
	http.HandleFunc("/job/lookup", lookup_job)
	http.HandleFunc("/job/update", update_job)
	http.HandleFunc("/job/notify", notify_job)
	http.HandleFunc("/job/kill", kill_job)
	http.HandleFunc("/job/signal", signal_job)
	http.HandleFunc("/job/complete", complete_job)
	http.HandleFunc("/job/suspend", suspend_job)
	http.HandleFunc("/job/resume", resume_job)
	http.HandleFunc("/job/requeue", requeue_job)

	http.HandleFunc("/job/step/kill", kill_job_step)
	http.HandleFunc("/job/step/signal", signal_job_step)
	http.HandleFunc("/job/step/terminate", terminate_job_step)

	/* TODO
	http.HandleFunc("/checkpoint/able", able_checkpoint)
	http.HandleFunc("/checkpoint/enable", enable_checkpoint)
	http.HandleFunc("/checkpoint/disable", disable_checkpoint)
	http.HandleFunc("/checkpoint/create", create_checkpoint)
	http.HandleFunc("/checkpoint/requeue", requeue_checkpoint)
	http.HandleFunc("/checkpoint/vacate", vacate_checkpoint)
	http.HandleFunc("/checkpoint/restart", restart_checkpoint)
	http.HandleFunc("/checkpoint/complete", complete_checkpoint)
	http.HandleFunc("/checkpoint/task/complete", task_complete_checkpoint)
	http.HandleFunc("/checkpoint/error", error_checkpoint)
	http.HandleFunc("/checkpoint/tasks", tasks_checkpoint)
	*/

	http.HandleFunc("/frontends", load_frontend)
	http.HandleFunc("/frontend/update", update_frontend)

	http.HandleFunc("/topologies", load_topo)

	http.HandleFunc("/partitions", load_partitions)
	http.HandleFunc("/partition/create", create_partition)
	http.HandleFunc("/partition/update", update_partition)
	http.HandleFunc("/partition/delete", delete_partition)

	http.HandleFunc("/reservations", load_reservations)
	http.HandleFunc("/reservation/create", create_reservation)
	http.HandleFunc("/reservation/update", update_reservation)
	http.HandleFunc("/reservation/delete", delete_reservation)

	http.HandleFunc("/triggers", get_triggers)
	http.HandleFunc("/trigger/create", set_trigger)
	http.HandleFunc("/trigger/delete", clear_trigger)

	http.HandleFunc("/ping", ping)
	http.HandleFunc("/reconfigure", reconfigure)
	http.HandleFunc("/shutdown", shutdown)
	http.HandleFunc("/takeover", takeover)

	log.Println("Listening...")

	// disable goroutine ?
	err = server.ListenAndServeTLS(*cert, *key)

	if err != nil {
		log.Fatal(err)
	}
}
