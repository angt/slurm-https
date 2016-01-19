git clone https://github.com/SchedMD/slurm
cd slurm

./autogen.sh
./configure --disable-dependency-tracking --prefix=/tmp
make install

mkdir -p ${PKG_CONFIG_PATH}
cat >${PKG_CONFIG_PATH}/slurm.pc <<EOF
includedir=/tmp/include
libdir=/tmp/lib
Cflags: -I\${includedir}
Libs: -L\${libdir} -lslurm
Description: Slurm API
$(grep Name META)
$(grep Version META)
EOF
