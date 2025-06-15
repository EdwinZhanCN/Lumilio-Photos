# 前端技术栈

## React.js
### Redux

## Vite
### Vitest

## TailwindCSS
### DaisyUI

## WebAssembly (WASM)
WebAssembly（简称 Wasm）是一种基于堆栈的虚拟机的二进制指令格式。Wasm 被设计为一种便携的编译目标，适用于编程语言，从而支持在客户端和服务器端的 Web 应用程序中部署。

更多信息请参见：https://webassembly.org

本项目使用`wasm-pack` 将**Rust**项目打包成`.wasm`和`.js`文件供前端**WebWorker**调用。

### BLAKE3 哈希
- **速度远快于** MD5、SHA-1、SHA-2、SHA-3 和 BLAKE2。
- **安全性高**，不像 MD5 和 SHA-1。而且不像 SHA-2，BLAKE3 不易受到长度扩展攻击。
- **高度并行化**，可跨任意数量的线程和 SIMD 通道运行，因为其内部是一个 Merkle 树。
- 支持 **验证流式处理** 和 **增量更新**，这同样得益于其内部的 Merkle 树结构。
- 既是一个 **PRF**、**MAC**、**KDF** 和 **XOF**，也是一个常规哈希算法。
- **单一算法，无需变体**，在 x86-64 和小型架构上均表现优异。

更多信息请参见：https://github.com/BLAKE3-team/BLAKE3

本项目在前端（客户端浏览器）中使用WebAssembly调用BLAKE3对用户上传的媒体进行哈希。

**重要方法**

`HashResult`

```js [src/wasm/blake3_wasm.js]
export class HashResult {
    // 封装哈希结果的类
    get hash() { /* 获取哈希字符串 */ }
    constructor(hash_string) { /* 从字符串创建实例 */ }
    free() { /* 释放 WASM 内存 */ }
}
```

`hash_asset`

```js [src/wasm/blake3_wasm.js]
export function hash_asset(buffer) {
    // 将二进制缓冲区传入 WASM 生成 BLAKE3 哈希
    // 返回 HashResult 对象
}
```

`compare_assets`

```js [src/wasm/blake3_wasm.js]
export function compare_assets(buffer1, buffer2) {
    // 直接比较两个二进制缓冲区的哈希是否相同
    // 返回布尔值
}
```

`verify_asset_hash`

```js [src/wasm/blake3_wasm.js]
export function verify_asset_hash(buffer, hash_string) {
    // 验证二进制缓冲区与已有哈希字符串是否匹配
    // 返回布尔值
}
```