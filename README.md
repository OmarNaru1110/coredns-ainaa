# ainaa

ainaa - provides intelligent DNS filtering with Redis caching and DynamoDB-backed protection.

## Description
The `ainaa` plugin integrates AinaaDNS filtering into CoreDNS. It performs intelligent DNS filtering decisions using a Redis cache for fast lookups and DynamoDB as the persistent protection backend. The plugin aims to block or allow queries based on AinaaDNS rules while caching recent results to reduce repeated external lookups.

## Compilation
This package is compiled as part of CoreDNS and not as a standalone binary. To use it you must add it as a plugin dependency in your CoreDNS `plugin.cfg` (or fetch it with `go get`) so that it is included when building CoreDNS.

Add the following to `plugin.cfg` (place early in the plugin list so `ainaa` runs before other plugins that may handle the query):

```
ainaa:github.com/OmarNaru1110/coredns-ainaa
```

Then recompile CoreDNS as usual:

```
go generate
go build
```

Or, if your environment provides it, you can use `make`:

```
make
```

The CoreDNS manual contains more information about configuring and extending the server with external plugins.

## Syntax
```
ainaa
```


## Examples
Use `ainaa` as the only plugin in the Corefile (listen on port 53):

```
.:53 {
    ainaa
}
```

Or enable `debug` before `ainaa` to get additional logging during processing:

```
.:53 {
    debug
    ainaa
}
```

## Notes
- Configure Redis and DynamoDB connection settings in the plugin's configuration (see source files for available flags and environment variables).
- Put `ainaa` early in your plugin list so it can make filtering decisions before other plugins respond.
