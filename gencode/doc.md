
Gateway-MCNM messages definition
===


<strong>Version 0.0.2</strong>

---
- [ApiVersionGet](#ApiVersionGet)
- [StatsGet](#StatsGet)
- [EventsWrite](#EventsWrite)
- [CommandSend](#CommandSend)
---
<a name="ApiVersionGet"></a>

## ApiVersionGet

Get supported versions

### Description
Retrieves the list of supported versions of this API.
### Result
| Name | Type | Constraints | Description |
| --- | --- | --- | --- |
| apiVersions | array |  | The API versions supported |
### Examples

#### Request

```json
{
    "id": 1,
    "jsonrpc": "2.0.0",
    "method": "ApiVersionGet"
}
```

#### Response

```json
{
    "id": 1,
    "jsonrpc": "2.0.0",
    "result": [
        "1.0.0",
        "0.0.1"
    ]
}
```
<a name="StatsGet"></a>

## StatsGet

Get statistics

### Description
Gets all the statistics records from the gateway.
### Parameters
| Name | Type | Constraints | Description |
| --- | --- | --- | --- |
| apiVersion | string | pattern:"^([0-9])\.([0-9])\.([0-9])?$" | The API version to be used |
### Result
| Name | Type | Constraints | Description |
| --- | --- | --- | --- |
| records | array |  | List of stats records retrieved |
### Examples

#### Request

```json
{
    "id": 1,
    "jsonrpc": "2.0.0",
    "method": "StatsGet",
    "params": {
        "ApiVersion": "0.0.2"
    }
}
```

#### Response

```json
{
    "id": 1,
    "jsonrpc": "2.0.0",
    "result": [
        {
            "element": "iface1",
            "stat": "pktsSent",
            "value": 10
        },
        {
            "element": "iface1",
            "stat": "pktsRcvd",
            "value": 20
        }
    ]
}
```
<a name="EventsWrite"></a>

## EventsWrite

Write events to the intercommunication bus

### Description
Write async events generated in the gateway to the intercommunication bus so they can be consumed.
### Parameters
| Name | Type | Constraints | Description |
| --- | --- | --- | --- |
| apiVersion | string | pattern:"^([0-9])\.([0-9])\.([0-9])?$" | The API version to be used |
### Result
| Name | Type | Constraints | Description |
| --- | --- | --- | --- |
| events | array |  | List of events to write |
### Examples

#### Request

```json
{
    "id": 1,
    "jsonrpc": "2.0.0",
    "method": "EventsWrite",
    "params": {
        "ApiVersion": "0.0.2"
    }
}
```

#### Response

```json
{
    "id": 1,
    "jsonrpc": "2.0.0",
    "result": [
        {
            "bgp": {
                "newState": "OpenConfirm",
                "oldState": "openSent",
                "peer": "100.0.0.25",
                "reason": "because"
            },
            "isis": {
                "level": "level-1-2",
                "newState": "initializing",
                "oldState": "up",
                "peer": "100.0.0.25",
                "reason": "because"
            }
        }
    ]
}
```
<a name="CommandSend"></a>

## CommandSend

Send a command to the gateway

### Description
Execute a command in the gateway and retrieve its output.
### Parameters
| Name | Type | Constraints | Description |
| --- | --- | --- | --- |
| apiVersion | string | pattern:"^([0-9])\.([0-9])\.([0-9])?$" | The API version to be used |
| cmd | string |  | Command to send |
| args | string |  | Command's arguments |
### Result
| Name | Type | Constraints | Description |
| --- | --- | --- | --- |
| output | object |  | Command's output. |
### Examples

#### Request

```json
{
    "id": 1,
    "jsonrpc": "2.0.0",
    "method": "CommandSend",
    "params": {
        "ApiVersion": "0.0.2",
        "args": "systemctl restart frr",
        "cmd": "shell"
    }
}
```

#### Response

```json
{
    "id": 1,
    "jsonrpc": "2.0.0",
    "result": [
        {
            "exitCode": 0,
            "stderr": null,
            "stdout": "frr restarted"
        }
    ]
}
```
