# server

gRPC server for ML prediction services.

This module initializes and serves a gRPC endpoint that delegates
prediction requests to registered model services, such as CLIP.

<a id="server.ModelRegistry"></a>

## ModelRegistry Objects

```python [pml/server.py]
class ModelRegistry()
```

Manage registration and lookup of ML model services.

**Attributes**:

- `services` - Mapping of service names to service instances.
- `start_time` - Timestamp when the registry was created.

<a id="server.ModelRegistry.register_service"></a>

#### register\_service

```python [pml/server.py]
def register_service(name: str, service: Any) -> None
```

Add a model service to the registry.

**Arguments**:

- `name` - Identifier for the service.
- `service` - Instance providing the service interface.

<a id="server.ModelRegistry.get_service"></a>

#### get\_service

```python [pml/server.py]
def get_service(name: str) -> Optional[Any]
```

Retrieve a registered service by name.

**Arguments**:

- `name` - Identifier of the service to fetch.


**Returns**:

  The service instance if found; otherwise None.

<a id="server.ModelRegistry.list_services"></a>

#### list\_services

```python [pml/server.py]
def list_services() -> list
```

List all registered service names.

**Returns**:

  A list of service identifiers.

<a id="server.ModelRegistry.get_uptime"></a>

#### get\_uptime

```python [pml/server.py]
def get_uptime() -> int
```

Calculate uptime since registry initialization.

**Returns**:

  Uptime in seconds.

<a id="server.PredictionServiceServicer"></a>

## PredictionServiceServicer Objects

```python [pml/server.py]
class PredictionServiceServicer(ml_service_pb2_grpc.PredictionServiceServicer)
```

gRPC servicer implementing prediction and health-check endpoints.

Delegates incoming calls to specific model services registered
in the ModelRegistry.

<a id="server.PredictionServiceServicer.ProcessImageForCLIP"></a>

#### ProcessImageForCLIP

```python [pml/server.py]
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

```python [pml/server.py]
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

```python [pml/server.py]
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

```python [pml/server.py]
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

```python [pml/server.py]
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

```python [pml/server.py]
def serve(port: int = 50051, max_workers: int = 10) -> None
```

Start the gRPC server for ML prediction.

**Arguments**:

- `port` - TCP port number to listen on.
- `max_workers` - Maximum number of thread pool workers.
