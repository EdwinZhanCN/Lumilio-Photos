<a id="proto.ml_service_pb2_grpc"></a>

# proto.ml\_service\_pb2\_grpc

Client and server classes corresponding to protobuf-defined services.

<a id="proto.ml_service_pb2_grpc.PredictionServiceStub"></a>

## PredictionServiceStub Objects

```python [pml/proto/ml_service_pb2.py]
class PredictionServiceStub(object)
```

PredictionService: 通用机器学习预测服务接口

<a id="proto.ml_service_pb2_grpc.PredictionServiceStub.__init__"></a>

#### \_\_init\_\_

```python [pml/proto/ml_service_pb2.py] [pml/proto/ml_service_pb2.py]
def __init__(channel)
```

Constructor.

**Arguments**:

- `channel` - A grpc.Channel.

<a id="proto.ml_service_pb2_grpc.PredictionServiceServicer"></a>

## PredictionServiceServicer Objects

```python [pml/proto/ml_service_pb2.py]
class PredictionServiceServicer(object)
```

PredictionService: 通用机器学习预测服务接口

<a id="proto.ml_service_pb2_grpc.PredictionServiceServicer.ProcessImageForCLIP"></a>

#### ProcessImageForCLIP

```python [pml/proto/ml_service_pb2.py]
def ProcessImageForCLIP(request, context)
```

ProcessImageForCLIP: 专为CLIP模型设计，处理图像并返回特征向量和可选标签

<a id="proto.ml_service_pb2_grpc.PredictionServiceServicer.GetTextEmbeddingForCLIP"></a>

#### GetTextEmbeddingForCLIP

```python [pml/proto/ml_service_pb2.py]
def GetTextEmbeddingForCLIP(request, context)
```

GetTextEmbeddingForCLIP: 专为CLIP模型设计，处理文本并返回特征向量

<a id="proto.ml_service_pb2_grpc.PredictionServiceServicer.Predict"></a>

#### Predict

```python [pml/proto/ml_service_pb2.py]
def Predict(request, context)
```

Predict: 通用预测接口，可用于未来其他模型的推理（如分类、回归等）
注意：这里的PredictRequest/Response可能需要根据具体模型调整

<a id="proto.ml_service_pb2_grpc.PredictionServiceServicer.BatchPredict"></a>

#### BatchPredict

```python [pml/proto/ml_service_pb2.py]
def BatchPredict(request, context)
```

BatchPredict: 批量通用预测接口

<a id="proto.ml_service_pb2_grpc.PredictionServiceServicer.HealthCheck"></a>

#### HealthCheck

```python [pml/proto/ml_service_pb2.py]
def HealthCheck(request, context)
```

HealthCheck: 模型状态检查，支持指定模型名称

<a id="proto.ml_service_pb2_grpc.PredictionService"></a>

## PredictionService Objects

```python [pml/proto/ml_service_pb2.py]
class PredictionService(object)
```

PredictionService: 通用机器学习预测服务接口

<a id="proto.ml_service_pb2"></a>

# proto.ml\_service\_pb2

Generated protocol buffer code.
