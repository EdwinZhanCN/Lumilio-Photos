#define VEC1_STATIC 1
#define sqlite3_extension_init sqlite3_vec1_extension_init

#if defined(__x86_64__) || defined(_M_X64)
#define VEC1SIMD SCALAR
#endif

#include "vec1.c"
