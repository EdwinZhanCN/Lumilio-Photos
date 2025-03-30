## portainer startup script

```shell
docker run -d -p 8000:8000 -p 9443:9443 --name portainer \
    --restart=always \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v portainer_data:/data \
    portainer/portainer-ce:latest
```

## efficientnet_v2 Image Classification

The ONNX model is converted from the TensorFlow model using the following command:

```python
from urllib.request import urlopen
from PIL import Image
import torch
import timm
import json

# Load image
img = Image.open(urlopen(
    'https://huggingface.co/datasets/huggingface/documentation-images/resolve/main/beignets-task-guide.png'
))

model = timm.create_model('tf_efficientnetv2_s.in21k_ft_in1k', pretrained=True)
model = model.eval()

# get model specific transforms (normalization, resize)
data_config = timm.data.resolve_model_data_config(model)
transforms = timm.data.create_transform(**data_config, is_training=False)

# Create a sample input tensor
x = transforms(img).unsqueeze(0)  # unsqueeze single image into batch of 1

# Run inference for testing
output = model(x)
top5_probabilities, top5_class_indices = torch.topk(output.softmax(dim=1) * 100, k=5)

# Load ImageNet class index from local file
with open('./imagenet_class_index.json', 'r') as f:
    imagenet_class_index = json.load(f)

# Map indices to class names
top5_classes = [imagenet_class_index[str(idx.item())][1] for idx in top5_class_indices[0]]
print(top5_classes)

# Export model to ONNX
torch.onnx.export(
    model,
    x,
    "efficientnet_v2.onnx",
    export_params=True,
    opset_version=12,
    do_constant_folding=True,
    input_names=["input"],
    output_names=["output"],
    dynamic_axes={
        "input": {0: "batch_size"},
        "output": {0: "batch_size"}
    }
)

print("Model exported to efficientnet_v2.onnx")
```