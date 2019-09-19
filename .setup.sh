git clone https://github.com/SchedMD/slurm
cd slurm

prefix=/tmp

./autogen.sh
./configure --disable-dependency-tracking --prefix=$prefix
make install -j

[ "$PKG_CONFIG_PATH" ] || PKG_CONFIG_PATH=$prefix/lib/pkgconfig

mkdir -p ${PKG_CONFIG_PATH}
cat >${PKG_CONFIG_PATH}/slurm.pc <<EOF
includedir=$prefix/include
libdir=$prefix/lib
Cflags: -I\${includedir}
Libs: -L\${libdir} -lslurm
Description: Slurm API
Name: slurm
Version: $(grep Version META | cut -f2)
EOF
