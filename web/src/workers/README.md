# Vitest For Web Workers

In this project, we use `@vitest/webworker` to test webworker functionality

## Example

```ts
import MyWorker from '../worker?worker'

let worker = new MyWorker()
// new Worker is also supported
worker = new Worker(new URL('../src/worker.ts', import.meta.url))

worker.postMessage('hello')
worker.onmessage = (e) => {
  // e.data equals to 'hello world'
}
```
