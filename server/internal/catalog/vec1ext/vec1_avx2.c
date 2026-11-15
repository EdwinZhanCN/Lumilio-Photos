#if defined(__x86_64__) || defined(_M_X64)

#if defined(__GNUC__) || defined(__clang__)
#pragma GCC push_options
#pragma GCC target("avx2,fma")
#endif

#define VEC1_STATIC 1
#define VEC1SIMD AVX2
#include "vec1.c"

#if defined(__GNUC__) || defined(__clang__)
#pragma GCC pop_options
#endif

#endif

