## ModelRegistry

```python
class ModelRegistry()
```

Manage registration and lookup of ML model services.

**Attributes**:

- `services` - Mapping of service names to service instances.
- `start_time` - Timestamp when the registry was created.

<a id="server.ModelRegistry.register_service"></a>

#### register\_service

```python
def register_service(name: str, service: Any) -> None
```

Add a model service to the registry.

**Arguments**:

- `name` - Identifier for the service.
- `service` - Instance providing the service interface.

<a id="server.ModelRegistry.get_service"></a>

#### get\_service

```python
def get_service(name: str) -> Optional[Any]
```

Retrieve a registered service by name.

**Arguments**:

- `name` - Identifier of the service to fetch.


**Returns**:

  The service instance if found; otherwise None.

<a id="server.ModelRegistry.list_services"></a>

#### list\_services

```python
def list_services() -> list
```

List all registered service names.

**Returns**:

  A list of service identifiers.

<a id="server.ModelRegistry.get_uptime"></a>

#### get\_uptime

```python
def get_uptime() -> int
```

Calculate uptime since registry initialization.

**Returns**:

  Uptime in seconds.
