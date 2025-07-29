<a id="image_classification.clip_model"></a>

# image\_classification.clip\_model

clip_model.py

Defines CLIPModelManager for loading an OpenCLIP model, preprocessing,
and performing inference tasks such as image encoding, text encoding,
classification, and similarity computation.

<a id="image_classification.clip_model.CLIPModelManager"></a>

## CLIPModelManager Objects

```python [pml/image-classification/*.py]
class CLIPModelManager()
```

Manage an OpenCLIP model and ImageNet class data for image classification.

**Attributes**:

- `model` - Loaded OpenCLIP model.
- `preprocess` - Preprocessing pipeline for image inputs.
- `tokenizer` - Tokenizer for text inputs.
- `imagenet_labels` - Mapping from class index to label info.
- `text_descriptions` - List of text prompts for classification.
- `model_name` - Name of the OpenCLIP architecture.
- `pretrained` - Identifier for pretrained weights.
- `imagenet_classes_path` - Optional path to custom ImageNet JSON.
- `load_time` - Time taken to load the model.
- `is_loaded` - Whether model and classes have been initialized.
- `device` - Torch device used for inference.

<a id="image_classification.clip_model.CLIPModelManager.__init__"></a>

#### \_\_init\_\_

```python [pml/image-classification/*.py]
def __init__(model_name: str = "ViT-B-32",
             pretrained: str = "laion2b_s34b_b79k",
             model_path: Optional[str] = None,
             imagenet_classes_path: Optional[str] = None) -> None
```

Initialize CLIPModelManager with configuration parameters.

**Arguments**:

- `model_name` - Architecture name to load.
- `pretrained` - Pretrained weight identifier.
- `model_path` - Optional local path to model checkpoint.
- `imagenet_classes_path` - Optional path to ImageNet class JSON.

<a id="image_classification.clip_model.CLIPModelManager.initialize"></a>

#### initialize

```python [pml/image-classification/*.py]
def initialize() -> None
```

Load the OpenCLIP model, transforms, tokenizer, and ImageNet classes.

**Raises**:

- `RuntimeError` - If model or classes fail to load.

<a id="image_classification.clip_model.CLIPModelManager.encode_image"></a>

#### encode\_image

```python [pml/image-classification/*.py]
def encode_image(image_bytes: bytes) -> np.ndarray
```

Encode raw image bytes into a unit-normalized feature vector.

**Arguments**:

- `image_bytes` - Raw image data in bytes.


**Returns**:

  1D numpy array of feature values.


**Raises**:

- `RuntimeError` - If model is not initialized.

<a id="image_classification.clip_model.CLIPModelManager.encode_text"></a>

#### encode\_text

```python [pml/image-classification/*.py]
def encode_text(text: str) -> np.ndarray
```

Encode text into a unit-normalized feature vector.

**Arguments**:

- `text` - Input text string.


**Returns**:

  1D numpy array of feature values.


**Raises**:

- `RuntimeError` - If model is not initialized.

<a id="image_classification.clip_model.CLIPModelManager.classify_image_with_labels"></a>

#### classify\_image\_with\_labels

```python [pml/image-classification/*.py]
def classify_image_with_labels(image_bytes: bytes,
                               target_labels: Optional[List[str]] = None,
                               top_k: int = 3) -> List[Tuple[str, float]]
```

Classify an image against specified or default labels.

**Arguments**:

- `image_bytes` - Raw image data in bytes.
- `target_labels` - Optional list of label names to restrict classification.
- `top_k` - Number of top predictions to return.


**Returns**:

  A list of (label, score) tuples for the top_k predictions.


**Raises**:

- `RuntimeError` - If model is not initialized.

<a id="image_classification.clip_model.CLIPModelManager.compute_similarity"></a>

#### compute\_similarity

```python [pml/image-classification/*.py]
def compute_similarity(image_bytes: bytes, text: str) -> float
```

Compute cosine similarity between an image and a text string.

**Arguments**:

- `image_bytes` - Raw image data in bytes.
- `text` - Text input for comparison.


**Returns**:

  Cosine similarity score.


**Raises**:

- `RuntimeError` - If model is not initialized.

<a id="image_classification.clip_model.CLIPModelManager.get_model_info"></a>

#### get\_model\_info

```python [pml/image-classification/*.py]
def get_model_info() -> Dict[str, Any]
```

Retrieve model configuration and status information.

**Returns**:

  Dictionary containing model_name, pretrained, device, load_time, and
  number of ImageNet classes loaded.

<a id="image_classification.clip_service"></a>

# image\_classification.clip\_service

clip_service.py

Provides gRPC service implementation for OpenCLIP operations, delegating
requests to a CLIPModelManager to perform image encoding, text embedding,
classification, similarity computation, health check, and model management.

<a id="image_classification.clip_service.CLIPService"></a>

## CLIPService Objects

```python [pml/image-classification/*.py]
class CLIPService()
```

gRPC service for OpenCLIP model operations.

Wraps a CLIPModelManager instance and exposes methods for:
  - Initializing the model
  - Image processing
  - Text embedding
  - Similarity computation
  - Health checks
  - Switching models at runtime
  - Retrieving service and model metadata

<a id="image_classification.clip_service.CLIPService.__init__"></a>

#### \_\_init\_\_

```python [pml/image-classification/*.py]
def __init__(model_name: str = "ViT-B-32",
             pretrained: str = "laion2b_s34b_b79k",
             model_path: Optional[str] = None,
             imagenet_classes_path: Optional[str] = None) -> None
```

Initialize the CLIPService.

**Arguments**:

- `model_name` - Name of the OpenCLIP architecture to load.
- `pretrained` - Identifier for pretrained weights.
- `model_path` - Optional path to a custom model checkpoint.
- `imagenet_classes_path` - Optional path to an ImageNet class index JSON.

<a id="image_classification.clip_service.CLIPService.initialize"></a>

#### initialize

```python [pml/image-classification/*.py]
def initialize() -> None
```

Load and initialize the OpenCLIP model and class data.

**Raises**:

- `RuntimeError` - If model initialization fails.

<a id="image_classification.clip_service.CLIPService.process_image_for_clip"></a>

#### process\_image\_for\_clip

```python [pml/image-classification/*.py]
def process_image_for_clip(request: ml_service_pb2.ImageProcessRequest,
                           context) -> ml_service_pb2.ImageProcessResponse
```

Handle an ImageProcessRequest via CLIPModelManager.

**Arguments**:

- `request` - Protobuf request containing image bytes, image_id,
  optional target_labels and top_k.
- `context` - gRPC context for setting status codes and details.


**Returns**:

  An ImageProcessResponse with feature vector, label scores,
  model_version, and processing_time_ms set. Status codes will
  be set on context for invalid input or internal errors.

<a id="image_classification.clip_service.CLIPService.get_text_embedding_for_clip"></a>

#### get\_text\_embedding\_for\_clip

```python [pml/image-classification/*.py]
def get_text_embedding_for_clip(
        request: ml_service_pb2.TextEmbeddingRequest,
        context) -> ml_service_pb2.TextEmbeddingResponse
```

Handle a TextEmbeddingRequest via CLIPModelManager.

**Arguments**:

- `request` - Protobuf request containing a non-empty text field.
- `context` - gRPC context for status codes.


**Returns**:

  A TextEmbeddingResponse with text_feature_vector, model_version,
  and processing_time_ms. Status codes set for invalid input.

<a id="image_classification.clip_service.CLIPService.compute_similarity_for_clip"></a>

#### compute\_similarity\_for\_clip

```python [pml/image-classification/*.py]
def compute_similarity_for_clip(image_bytes: bytes, text: str) -> float
```

Compute cosine similarity between image and text embeddings.

**Arguments**:

- `image_bytes` - Raw image bytes.
- `text` - Input text string.


**Returns**:

  Cosine similarity score as a float.


**Raises**:

- `RuntimeError` - If the model is not initialized.

<a id="image_classification.clip_service.CLIPService.health_check"></a>

#### health\_check

```python [pml/image-classification/*.py]
def health_check(
        service_name: str = "openclip") -> ml_service_pb2.HealthCheckResponse
```

Perform a health check of the CLIPService.

**Arguments**:

- `service_name` - Identifier returned in the response.


**Returns**:

  A HealthCheckResponse with status, model_name, model_version,
  uptime_seconds, and a descriptive message.

<a id="image_classification.clip_service.CLIPService.get_model_info"></a>

#### get\_model\_info

```python [pml/image-classification/*.py]
def get_model_info() -> Dict[str, Any]
```

Retrieve metadata about the current model and service.

**Returns**:

  A dictionary containing model_name, pretrained weights,
  device, is_loaded, load_time, and service uptime.

<a id="image_classification.clip_service.CLIPService.switch_model"></a>

#### switch\_model

```python [pml/image-classification/*.py]
def switch_model(new_model_name: str,
                 new_pretrained: str,
                 model_path: Optional[str] = None) -> bool
```

Replace the current CLIP model with a new one at runtime.

**Arguments**:

- `new_model_name` - Name of the new model architecture.
- `new_pretrained` - Identifier for the new pretrained weights.
- `model_path` - Optional checkpoint path for the new model.


**Returns**:

  True if the switch succeeds; raises on failure.

<a id="image_classification.clip_service.CLIPService.get_performance_stats"></a>

#### get\_performance\_stats

```python [pml/image-classification/*.py]
def get_performance_stats() -> Dict[str, Any]
```

Gather runtime performance statistics for the service.

**Returns**:

  A dictionary with model, device, uptime, load_time, and health status.
