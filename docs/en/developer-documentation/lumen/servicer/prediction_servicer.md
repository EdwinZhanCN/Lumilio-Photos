## PredictionServiceServicer

```python
class PredictionServiceServicer(ml_service_pb2_grpc.PredictionServiceServicer)
```

gRPC servicer implementing prediction and health-check endpoints.

Delegates incoming calls to specific model services registered
in the ModelRegistry.

<a id="server.PredictionServiceServicer.ProcessImageForCLIP"></a>

#### ProcessImageForCLIP

```python
def ProcessImageForCLIP(request, context)
```

Handle image processing requests for CLIP.

**Arguments**:

- `request` - Protobuf request containing raw image bytes.
- `context` - gRPC context for status and metadata.


**Returns**:

  ImageProcessResponse with processed image features.

<a id="server.PredictionServiceServicer.GetTextEmbeddingForCLIP"></a>

#### GetTextEmbeddingForCLIP

```python
def GetTextEmbeddingForCLIP(request, context)
```

Handle text embedding requests for CLIP.

**Arguments**:

- `request` - Protobuf request containing text input.
- `context` - gRPC context for status and metadata.


**Returns**:

  TextEmbeddingResponse with embedding vector.

<a id="server.PredictionServiceServicer.Predict"></a>

#### Predict

```python
def Predict(request, context)
```

Generic prediction endpoint supporting multiple models.

Dispatches based on request.model_name. Supports CLIP for
both image and text inputs.

**Arguments**:

- `request` - PredictRequest containing model name and input data.
- `context` - gRPC context for status and metadata.


**Returns**:

  PredictResponse with float values and metadata.

<a id="server.PredictionServiceServicer.BatchPredict"></a>

#### BatchPredict

```python
def BatchPredict(request, context)
```

Process a batch of prediction requests.

**Arguments**:

- `request` - BatchPredictRequest with multiple PredictRequests.
- `context` - gRPC context for status and metadata.


**Returns**:

  BatchPredictResponse summarizing successes and failures.

<a id="server.PredictionServiceServicer.HealthCheck"></a>

#### HealthCheck

```python
def HealthCheck(request, context)
```

Health check endpoint for individual or all services.

**Arguments**:

- `request` - HealthCheckRequest specifying a service name or 'all'.
- `context` - gRPC context for status and metadata.


**Returns**:

  HealthCheckResponse with status, uptime, and message.

<a id="server.serve"></a>

#### serve

```python
def serve(port: int = 50051, max_workers: int = 10) -> None
```

Start the gRPC server for ML prediction.

**Arguments**:

- `port` - TCP port number to listen on.
- `max_workers` - Maximum number of thread pool workers.
