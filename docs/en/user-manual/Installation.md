# Installation Guide

## Configurations

### Minimal (极简)

```json
{
    "ocr":{
		"model" : "PP-OCRv5", // 对应huggingface仓库 Lumilio-Photos/PP-OCRv5, 对应modelscope仓库 LumilioPhotos/PP-OCRv5
		"runtime": "onnx" | "rknn",
		"rknn_device": "none" | "rk3588" // 可选，或者其他芯片
	}
}
```

###  LightWeight （轻量）

对于内存 >= 4GB 且没有独立GPU的，例如 `Intel N100` , 包含标准CLIP, Face Recognition和OCR服务。

```json
{
	"region": "cn" | "other", // cn 选用modelscope作为platform, 其余为huggingface
	"clip":{
		"model" : "MobileCLIP-B" | "CN-CLIP_ViT-B/16", // 对应huggingface仓库 Lumilio-Photos/MobileCLIP-B, 对应modelscope仓库 LumilioPhotos/MobileCLIP-B, Chinese CLIP 类似
		"runtime": "torch" | "onnx" | "rknn",
		"rknn_device": "none" | "rk3588" // 可选，或者其他芯片
	},
	"face":{
		"model" : "buffalo_l", // 对应huggingface仓库 Lumilio-Photos/buffalo_l, 对应modelscope仓库 LumilioPhotos/buffalo_l
		"runtime": "onnx" | "rknn",
		"rknn_device": "none" | "rk3588" // 可选，或者其他芯片
	},
	"ocr":{
		"model" : "PP-OCRv5", // 对应huggingface仓库 Lumilio-Photos/PP-OCRv5, 对应modelscope仓库 LumilioPhotos/PP-OCRv5
		"runtime": "onnx" | "rknn",
		"rknn_device": "none" | "rk3588" // 可选，或者其他芯片
	}
}
```

### Basic （基础）

包含所有feature服务，新增BioCLIP用于生物识别。

```json
{
	"region": "cn" | "other", // cn 选用modelscope作为platform, 其余为huggingface
	"clip":{
		"model" : "MobileCLIP-B" | "CN-CLIP_ViT-B/16", // 对应huggingface仓库 Lumilio-Photos/MobileCLIP-B, 对应modelscope仓库 LumilioPhotos/MobileCLIP-B, Chinese CLIP 类似
		"runtime": "torch" | "onnx" | "rknn",
		"rknn_device": "none" | "rk3588" // 可选，或者其他芯片
	},
	"face":{
		"model" : "buffalo_l", // 对应huggingface仓库 Lumilio-Photos/buffalo_l, 对应modelscope仓库 LumilioPhotos/buffalo_l
		"runtime": "onnx" | "rknn",
		"rknn_device": "none" | "rk3588" // 可选，或者其他芯片
	},
	"ocr":{
		"model" : "PP-OCRv5", // 对应huggingface仓库 Lumilio-Photos/PP-OCRv5, 对应modelscope仓库 LumilioPhotos/PP-OCRv5
		"runtime": "onnx" | "rknn",
		"rknn_device": "none" | "rk3588" // 可选，或者其他芯片
	},
	"bioclip":{
		"model" : "bioclip-2", // 对应huggingface仓库 Lumilio-Photos/bioclip-2, 对应modelscope仓库 LumilioPhotos/bioclip-2
		"runtime": "torch" | "onnx" | "rknn",
		"rknn_device": "none" | "rk3588", // 可选，或者其他芯片
		"dataset": "TreeOfLife-10M" // 对应bioclip-2仓库下的TreeOfLife-10M.npz
	}
}
```

### Brave (激进)

包含所有feature增强版

```json
{
	"region": "cn" | "other", // cn 选用modelscope作为platform, 其余为huggingface
	"clip":{
		"model" : "MobileCLIP-L-14" | "CN-CLIPViT-L/14", // 对应huggingface仓库 Lumilio-Photos/MobileCLIP-L-14, 对应modelscope仓库 LumilioPhotos/MobileCLIP-L-14, Chinese CLIP 类似
		"runtime": "torch" | "onnx" | "rknn",
		"rknn_device": "none" | "rk3588" // 可选，或者其他芯片
	},
	"face":{
		"model" : "antelopev2", // 对应huggingface仓库 Lumilio-Photos/antelopev2, 对应modelscope仓库 LumilioPhotos/antelopev2
		"runtime": "onnx" | "rknn",
		"rknn_device": "none" | "rk3588" // 可选，或者其他芯片
	},
	"ocr":{
		"model" : "PP-OCRv5", // 对应huggingface仓库 Lumilio-Photos/PP-OCRv5, 对应modelscope仓库 LumilioPhotos/PP-OCRv5
		"runtime": "onnx" | "rknn",
		"rknn_device": "none" | "rk3588" // 可选，或者其他芯片
	},
	"bioclip":{
		"model" : "bioclip-2", // 对应huggingface仓库 Lumilio-Photos/bioclip-2, 对应modelscope仓库 LumilioPhotos/bioclip-2
		"runtime": "torch" | "onnx" | "rknn",
		"rknn_device": "none" | "rk3588", // 可选，或者其他芯片
		"dataset": "TreeOfLife-200M" // 对应bioclip-2仓库下的TreeOfLife-200M.npz
	}
}
```
