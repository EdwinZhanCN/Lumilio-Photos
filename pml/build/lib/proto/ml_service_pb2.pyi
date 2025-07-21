from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class LabelScore(_message.Message):
    __slots__ = ("label", "similarity_score")
    LABEL_FIELD_NUMBER: _ClassVar[int]
    SIMILARITY_SCORE_FIELD_NUMBER: _ClassVar[int]
    label: str
    similarity_score: float
    def __init__(self, label: _Optional[str] = ..., similarity_score: _Optional[float] = ...) -> None: ...

class ImageProcessRequest(_message.Message):
    __slots__ = ("image_id", "image_data", "target_labels", "model_version")
    IMAGE_ID_FIELD_NUMBER: _ClassVar[int]
    IMAGE_DATA_FIELD_NUMBER: _ClassVar[int]
    TARGET_LABELS_FIELD_NUMBER: _ClassVar[int]
    MODEL_VERSION_FIELD_NUMBER: _ClassVar[int]
    image_id: str
    image_data: bytes
    target_labels: _containers.RepeatedScalarFieldContainer[str]
    model_version: str
    def __init__(self, image_id: _Optional[str] = ..., image_data: _Optional[bytes] = ..., target_labels: _Optional[_Iterable[str]] = ..., model_version: _Optional[str] = ...) -> None: ...

class ImageProcessResponse(_message.Message):
    __slots__ = ("image_id", "image_feature_vector", "predicted_scores", "model_version", "processing_time_ms")
    IMAGE_ID_FIELD_NUMBER: _ClassVar[int]
    IMAGE_FEATURE_VECTOR_FIELD_NUMBER: _ClassVar[int]
    PREDICTED_SCORES_FIELD_NUMBER: _ClassVar[int]
    MODEL_VERSION_FIELD_NUMBER: _ClassVar[int]
    PROCESSING_TIME_MS_FIELD_NUMBER: _ClassVar[int]
    image_id: str
    image_feature_vector: _containers.RepeatedScalarFieldContainer[float]
    predicted_scores: _containers.RepeatedCompositeFieldContainer[LabelScore]
    model_version: str
    processing_time_ms: int
    def __init__(self, image_id: _Optional[str] = ..., image_feature_vector: _Optional[_Iterable[float]] = ..., predicted_scores: _Optional[_Iterable[_Union[LabelScore, _Mapping]]] = ..., model_version: _Optional[str] = ..., processing_time_ms: _Optional[int] = ...) -> None: ...

class TextEmbeddingRequest(_message.Message):
    __slots__ = ("text", "model_version")
    TEXT_FIELD_NUMBER: _ClassVar[int]
    MODEL_VERSION_FIELD_NUMBER: _ClassVar[int]
    text: str
    model_version: str
    def __init__(self, text: _Optional[str] = ..., model_version: _Optional[str] = ...) -> None: ...

class TextEmbeddingResponse(_message.Message):
    __slots__ = ("text_feature_vector", "model_version", "processing_time_ms")
    TEXT_FEATURE_VECTOR_FIELD_NUMBER: _ClassVar[int]
    MODEL_VERSION_FIELD_NUMBER: _ClassVar[int]
    PROCESSING_TIME_MS_FIELD_NUMBER: _ClassVar[int]
    text_feature_vector: _containers.RepeatedScalarFieldContainer[float]
    model_version: str
    processing_time_ms: int
    def __init__(self, text_feature_vector: _Optional[_Iterable[float]] = ..., model_version: _Optional[str] = ..., processing_time_ms: _Optional[int] = ...) -> None: ...

class PredictRequest(_message.Message):
    __slots__ = ("raw_data", "float_features", "text_input", "model_name", "model_version")
    RAW_DATA_FIELD_NUMBER: _ClassVar[int]
    FLOAT_FEATURES_FIELD_NUMBER: _ClassVar[int]
    TEXT_INPUT_FIELD_NUMBER: _ClassVar[int]
    MODEL_NAME_FIELD_NUMBER: _ClassVar[int]
    MODEL_VERSION_FIELD_NUMBER: _ClassVar[int]
    raw_data: bytes
    float_features: FloatFeatures
    text_input: str
    model_name: str
    model_version: str
    def __init__(self, raw_data: _Optional[bytes] = ..., float_features: _Optional[_Union[FloatFeatures, _Mapping]] = ..., text_input: _Optional[str] = ..., model_name: _Optional[str] = ..., model_version: _Optional[str] = ...) -> None: ...

class PredictResponse(_message.Message):
    __slots__ = ("prediction_floats", "prediction_text", "confidence", "model_name", "model_version", "prediction_time_ms")
    PREDICTION_FLOATS_FIELD_NUMBER: _ClassVar[int]
    PREDICTION_TEXT_FIELD_NUMBER: _ClassVar[int]
    CONFIDENCE_FIELD_NUMBER: _ClassVar[int]
    MODEL_NAME_FIELD_NUMBER: _ClassVar[int]
    MODEL_VERSION_FIELD_NUMBER: _ClassVar[int]
    PREDICTION_TIME_MS_FIELD_NUMBER: _ClassVar[int]
    prediction_floats: PredictionFloats
    prediction_text: str
    confidence: float
    model_name: str
    model_version: str
    prediction_time_ms: int
    def __init__(self, prediction_floats: _Optional[_Union[PredictionFloats, _Mapping]] = ..., prediction_text: _Optional[str] = ..., confidence: _Optional[float] = ..., model_name: _Optional[str] = ..., model_version: _Optional[str] = ..., prediction_time_ms: _Optional[int] = ...) -> None: ...

class BatchPredictRequest(_message.Message):
    __slots__ = ("requests", "model_name")
    REQUESTS_FIELD_NUMBER: _ClassVar[int]
    MODEL_NAME_FIELD_NUMBER: _ClassVar[int]
    requests: _containers.RepeatedCompositeFieldContainer[PredictRequest]
    model_name: str
    def __init__(self, requests: _Optional[_Iterable[_Union[PredictRequest, _Mapping]]] = ..., model_name: _Optional[str] = ...) -> None: ...

class BatchPredictResponse(_message.Message):
    __slots__ = ("responses", "success_count", "failed_count")
    RESPONSES_FIELD_NUMBER: _ClassVar[int]
    SUCCESS_COUNT_FIELD_NUMBER: _ClassVar[int]
    FAILED_COUNT_FIELD_NUMBER: _ClassVar[int]
    responses: _containers.RepeatedCompositeFieldContainer[PredictResponse]
    success_count: int
    failed_count: int
    def __init__(self, responses: _Optional[_Iterable[_Union[PredictResponse, _Mapping]]] = ..., success_count: _Optional[int] = ..., failed_count: _Optional[int] = ...) -> None: ...

class FloatFeatures(_message.Message):
    __slots__ = ("values",)
    VALUES_FIELD_NUMBER: _ClassVar[int]
    values: _containers.RepeatedScalarFieldContainer[float]
    def __init__(self, values: _Optional[_Iterable[float]] = ...) -> None: ...

class PredictionFloats(_message.Message):
    __slots__ = ("values",)
    VALUES_FIELD_NUMBER: _ClassVar[int]
    values: _containers.RepeatedScalarFieldContainer[float]
    def __init__(self, values: _Optional[_Iterable[float]] = ...) -> None: ...

class HealthCheckRequest(_message.Message):
    __slots__ = ("service_name",)
    SERVICE_NAME_FIELD_NUMBER: _ClassVar[int]
    service_name: str
    def __init__(self, service_name: _Optional[str] = ...) -> None: ...

class HealthCheckResponse(_message.Message):
    __slots__ = ("status", "model_name", "model_version", "uptime_seconds", "message")
    class ServingStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        UNKNOWN: _ClassVar[HealthCheckResponse.ServingStatus]
        SERVING: _ClassVar[HealthCheckResponse.ServingStatus]
        NOT_SERVING: _ClassVar[HealthCheckResponse.ServingStatus]
        SERVICE_SPECIFIC_ERROR: _ClassVar[HealthCheckResponse.ServingStatus]
    UNKNOWN: HealthCheckResponse.ServingStatus
    SERVING: HealthCheckResponse.ServingStatus
    NOT_SERVING: HealthCheckResponse.ServingStatus
    SERVICE_SPECIFIC_ERROR: HealthCheckResponse.ServingStatus
    STATUS_FIELD_NUMBER: _ClassVar[int]
    MODEL_NAME_FIELD_NUMBER: _ClassVar[int]
    MODEL_VERSION_FIELD_NUMBER: _ClassVar[int]
    UPTIME_SECONDS_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    status: HealthCheckResponse.ServingStatus
    model_name: str
    model_version: str
    uptime_seconds: int
    message: str
    def __init__(self, status: _Optional[_Union[HealthCheckResponse.ServingStatus, str]] = ..., model_name: _Optional[str] = ..., model_version: _Optional[str] = ..., uptime_seconds: _Optional[int] = ..., message: _Optional[str] = ...) -> None: ...
