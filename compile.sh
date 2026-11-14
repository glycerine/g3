#!/bin/bash
set -e

# 1. Wipe the CMake build directory

# remove the wasm.build symlink, if present (compile-wasm.sh creates).
rm -rf build

# 2. Run the legacy Python generator, explicitly requesting a static library.
# (We pass the "silence the sprintf blabber" flags via standard environment variables here)
CXXFLAGS="-Wno-deprecated-declarations" CFLAGS="-Wno-deprecated-declarations" python3 scripts/mk_make.py --staticlib

# 3. The script creates a new 'build' directory. Go into it.
cd build

# 4. Compile using standard Make (not CMake)
make -j8

# 5. Copy the artifact out so wasm can build too.
#cp -p libz3.a ../../native_lib/
#cp -p libz3.so ../../native_lib/
#cp -p z3 ../../native_lib/

#cd ..
#rm -rf native.build
#mv build native.build

# 6. build wasm too, using emscripten
#./compile-wasm.sh
