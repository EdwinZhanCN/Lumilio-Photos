from google.protobuf import empty_pb2 as _empty_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ErrorCode(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    ERROR_CODE_UNSPECIFIED: _ClassVar[ErrorCode]
    ERROR_CODE_INVALID_ARGUMENT: _ClassVar[ErrorCode]
    ERROR_CODE_UNAVAILABLE: _ClassVar[ErrorCode]
    ERROR_CODE_DEADLINE_EXCEEDED: _ClassVar[ErrorCode]
    ERROR_CODE_INTERNAL: _ClassVar[ErrorCode]
ERROR_CODE_UNSPECIFIED: ErrorCode
ERROR_CODE_INVALID_ARGUMENT: ErrorCode
ERROR_CODE_UNAVAILABLE: ErrorCode
ERROR_CODE_DEADLINE_EXCEEDED: ErrorCode
ERROR_CODE_INTERNAL: ErrorCode

class Error(_message.Message):
    __slots__ = ("code", "message", "detail")
    CODE_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    DETAIL_FIELD_NUMBER: _ClassVar[int]
    code: ErrorCode
    message: str
    detail: str
    def __init__(self, code: _Optional[_Union[ErrorCode, str]] = ..., message: _Optional[str] = ..., detail: _Optional[str] = ...) -> None: ...

class IOTask(_message.Message):
    __slots__ = ("name", "input_mimes", "output_mimes", "limits")
    class LimitsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    INPUT_MIMES_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_MIMES_FIELD_NUMBER: _ClassVar[int]
    LIMITS_FIELD_NUMBER: _ClassVar[int]
    name: str
    input_mimes: _containers.RepeatedScalarFieldContainer[str]
    output_mimes: _containers.RepeatedScalarFieldContainer[str]
    limits: _containers.ScalarMap[str, str]
    def __init__(self, name: _Optional[str] = ..., input_mimes: _Optional[_Iterable[str]] = ..., output_mimes: _Optional[_Iterable[str]] = ..., limits: _Optional[_Mapping[str, str]] = ...) -> None: ...

class Capability(_message.Message):
    __slots__ = ("service_name", "model_ids", "runtime", "max_concurrency", "precisions", "extra", "tasks")
    class ExtraEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    SERVICE_NAME_FIELD_NUMBER: _ClassVar[int]
    MODEL_IDS_FIELD_NUMBER: _ClassVar[int]
    RUNTIME_FIELD_NUMBER: _ClassVar[int]
    MAX_CONCURRENCY_FIELD_NUMBER: _ClassVar[int]
    PRECISIONS_FIELD_NUMBER: _ClassVar[int]
    EXTRA_FIELD_NUMBER: _ClassVar[int]
    TASKS_FIELD_NUMBER: _ClassVar[int]
    service_name: str
    model_ids: _containers.RepeatedScalarFieldContainer[str]
    runtime: str
    max_concurrency: int
    precisions: _containers.RepeatedScalarFieldContainer[str]
    extra: _containers.ScalarMap[str, str]
    tasks: _containers.RepeatedCompositeFieldContainer[IOTask]
    def __init__(self, service_name: _Optional[str] = ..., model_ids: _Optional[_Iterable[str]] = ..., runtime: _Optional[str] = ..., max_concurrency: _Optional[int] = ..., precisions: _Optional[_Iterable[str]] = ..., extra: _Optional[_Mapping[str, str]] = ..., tasks: _Optional[_Iterable[_Union[IOTask, _Mapping]]] = ...) -> None: ...

class InferRequest(_message.Message):
    __slots__ = ("correlation_id", "task", "payload", "meta", "payload_mime", "seq", "total", "offset")
    class MetaEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    CORRELATION_ID_FIELD_NUMBER: _ClassVar[int]
    TASK_FIELD_NUMBER: _ClassVar[int]
    PAYLOAD_FIELD_NUMBER: _ClassVar[int]
    META_FIELD_NUMBER: _ClassVar[int]
    PAYLOAD_MIME_FIELD_NUMBER: _ClassVar[int]
    SEQ_FIELD_NUMBER: _ClassVar[int]
    TOTAL_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    correlation_id: str
    task: str
    payload: bytes
    meta: _containers.ScalarMap[str, str]
    payload_mime: str
    seq: int
    total: int
    offset: int
    def __init__(self, correlation_id: _Optional[str] = ..., task: _Optional[str] = ..., payload: _Optional[bytes] = ..., meta: _Optional[_Mapping[str, str]] = ..., payload_mime: _Optional[str] = ..., seq: _Optional[int] = ..., total: _Optional[int] = ..., offset: _Optional[int] = ...) -> None: ...

class InferResponse(_message.Message):
    __slots__ = ("correlation_id", "is_final", "result", "meta", "error", "seq", "total", "offset", "result_mime", "result_schema")
    class MetaEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    CORRELATION_ID_FIELD_NUMBER: _ClassVar[int]
    IS_FINAL_FIELD_NUMBER: _ClassVar[int]
    RESULT_FIELD_NUMBER: _ClassVar[int]
    META_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    SEQ_FIELD_NUMBER: _ClassVar[int]
    TOTAL_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    RESULT_MIME_FIELD_NUMBER: _ClassVar[int]
    RESULT_SCHEMA_FIELD_NUMBER: _ClassVar[int]
    correlation_id: str
    is_final: bool
    result: bytes
    meta: _containers.ScalarMap[str, str]
    error: Error
    seq: int
    total: int
    offset: int
    result_mime: str
    result_schema: str
    def __init__(self, correlation_id: _Optional[str] = ..., is_final: bool = ..., result: _Optional[bytes] = ..., meta: _Optional[_Mapping[str, str]] = ..., error: _Optional[_Union[Error, _Mapping]] = ..., seq: _Optional[int] = ..., total: _Optional[int] = ..., offset: _Optional[int] = ..., result_mime: _Optional[str] = ..., result_schema: _Optional[str] = ...) -> None: ...
