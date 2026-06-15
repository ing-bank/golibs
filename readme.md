# ING Container Hosting Platform Go Libraries

## About

GoLibs is a collection of reusable Go packages for building robust, testable, and maintainable applications. It provides
utilities for configuration, HTTP clients/servers, logging, graceful shutdown, key-value stores, Kubernetes testing, and
more.

The initial open-source release of this project is provided as-is. That implies that the codebase is only a slightly
modified version of what we at ING are using. In the future we would like to extend this Open Source repository with
the appropriate pipelines and contribution mechanics. This does imply that your journey for using this project for your
own purposes can use some improvement, and we will work on that.

This project is part of ING Neoria. Neoria contains parts of the ING Container Hosting Platform (ICHP) stack
which is used to deliver Namespace-as-a-Service on top of OpenShift.

## Installation

```sh
go get github.com/ing-bank/golibs
```

## Packages

| Package                                                   | Description                                                                                                                                         |
|-----------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------|
| [`access`](pkg/access)                                    | Subject Access Reviews and authorization helpers based on trust set in request contexts. Used for fine-grained access control in APIs and services. |
| [`scope`](pkg/access/scope)                               | Scope definitions and utilities for expressing permissions and access levels.                                                                       |
| [`config`](pkg/config)                                    | Load, validate, and apply defaults to configuration structs from YAML/JSON files. Supports command-line flag binding.                               |
| [`defaultmap`](pkg/defaultmap)                            | map with key collision handling, merging, and update logic.                                                                                         |
| [`errors`](pkg/errors)                                    | HTTP status code error types, conversion utilities, and retry logic for API/transport errors. Includes helpers for common error patterns.           |
| [`fsnotify`](pkg/fsnotify)                                | Polling-based file watcher for monitoring file changes and triggering callbacks, useful for hot-reload scenarios.                                   |
| [`ginresponse`](pkg/ginresponse)                          | Experimental: Response helpers for Gin HTTP handlers, simplifying response formatting and error handling.                                           |
| [`ginserver`](pkg/ginserver)                              | Utilities for setting up Gin-based HTTP servers with middleware, metrics, graceful shutdown, and advanced configuration.                            |
| [`manager`](pkg/ginserver/manager)                        | Gin server manager for orchestrating server lifecycle and configuration.                                                                            |
| [`proxy`](pkg/ginserver/proxy)                            | Reverse proxy middleware for Gin servers, enabling request forwarding and load balancing.                                                           |
| [`graceful`](pkg/graceful)                                | Graceful shutdown and background task management with context cancellation, signal handling, and concurrency support.                               |
| [`healthcheck`](pkg/healthcheck)                          | Health check endpoints and checks for HTTP servers, background jobs, and dependencies.                                                              |
| [`checks`](pkg/healthcheck/checks)                        | Built-in health checks for common protocols and services.                                                                                           |
| [`http`](pkg/healthcheck/checks/http)                     | HTTP endpoint health checks.                                                                                                                        |
| [`ok`](pkg/healthcheck/checks/ok)                         | Always-OK health checks for basic liveness probes.                                                                                                  |
| [`telnet`](pkg/healthcheck/checks/telnet)                 | Telnet-based health checks for legacy systems.                                                                                                      |
| [`http`](pkg/http)                                        | HTTP client wrapper with middleware (tripperware), transport customization, and simplified request/response handling.                               |
| [`response`](pkg/http/response)                           | HTTP response helpers for parsing and error handling.                                                                                               |
| [`tripperware`](pkg/http/tripperware)                     | HTTP client middleware for metrics, retries, logging, and more.                                                                                     |
| [`httpserver`](pkg/httpserver)                            | HTTP server utilities with graceful shutdown, configuration, and middleware support.                                                                |
| [`iofs`](pkg/iofs)                                        | Experimental: Generic utilities for reading and parsing files from various file systems, with format conversion helpers.                            |
| [`kafka`](pkg/kafka)                                      | Kafka client utilities for producing, consuming, and monitoring Kafka messages.                                                                     |
| [`consumer`](pkg/kafka/consumer)                          | Kafka consumer helpers and configuration.                                                                                                           |
| [`errors`](pkg/kafka/errors)                              | Kafka error helpers and error type definitions.                                                                                                     |
| [`producer`](pkg/kafka/producer)                          | Kafka producer helpers and configuration.                                                                                                           |
| [`stats`](pkg/kafka/stats)                                | Kafka statistics collection and monitoring utilities.                                                                                               |
| [`kubemock`](pkg/kubemock)                                | Utilities for testing Kubernetes event-based applications with a fake client, supporting dry-run and error simulation.                              |
| [`logging`](pkg/logging)                                  | Structured logging with request tracing, logrus integration, and log data truncation.                                                               |
| [`middleware`](pkg/middleware)                            | HTTP middleware utilities for authorization, compression, logging, metrics, and more.                                                               |
| [`authorization`](pkg/middleware/authorization)           | Authorization middleware for various authentication strategies (certificate, NPA, OAuth, trust, user).                                              |
| [`certificate`](pkg/middleware/authorization/certificate) | Certificate-based authentication middleware.                                                                                                        |
| [`npa`](pkg/middleware/authorization/npa)                 | NPA-based authentication middleware.                                                                                                                |
| [`oauth`](pkg/middleware/authorization/oauth)             | OAuth-based authentication middleware.                                                                                                              |
| [`trust`](pkg/middleware/authorization/trust)             | Trust-based authentication middleware.                                                                                                              |
| [`user`](pkg/middleware/authorization/user)               | User-based authentication middleware.                                                                                                               |
| [`gzip`](pkg/middleware/gzip)                             | GZIP compression middleware for HTTP servers.                                                                                                       |
| [`logger`](pkg/middleware/logger)                         | Logging middleware for HTTP requests.                                                                                                               |
| [`metrics`](pkg/middleware/metrics)                       | Prometheus metrics middleware for HTTP servers.                                                                                                     |
| [`requestid`](pkg/middleware/requestid)                   | Middleware for injecting and propagating request IDs.                                                                                               |
| [`timeout`](pkg/middleware/timeout)                       | Middleware for enforcing request timeouts.                                                                                                          |
| [`opt`](pkg/opt)                                          | Utilities for handling optional parameters with sensible defaults, emulating optional arguments in Go.                                              |
| [`orchestration`](pkg/orchestration)                      | Orchestration utilities for workflow-based resource management and automation.                                                                      |
| [`audit`](pkg/orchestration/audit)                        | Audit logging for orchestration workflows.                                                                                                          |
| [`service`](pkg/orchestration/service)                    | Service interface for workflow-based resource orchestration, supporting validation, apply, and delete actions.                                      |
| [`status`](pkg/orchestration/status)                      | Status helpers for orchestration and workflow tracking.                                                                                             |
| [`patch`](pkg/patch)                                      | Utilities for applying JSON Patch and Merge Patch operations to Go structs, supporting RFC 6902/7386.                                               |
| [`reloader`](pkg/reloader)                                | Watches files and reloads Kubernetes deployments on changes, supporting hot-reload of certificates and configs.                                     |
| [`retry`](pkg/retry)                                      | Retry logic and backoff strategies for operations, with customizable policies.                                                                      |
| [`slices`](pkg/slices)                                    | Generic utilities for working with slices, including unique, merge, and threadsafe operations.                                                      |
| [`store`](pkg/store)                                      | Generic interfaces and implementations for key-value stores, with pluggable backends and middleware.                                                |
| [`backends`](pkg/store/backends)                          | Store backend implementations for various storage systems.                                                                                          |
| [`configmap`](pkg/store/backends/configmap)               | Kubernetes ConfigMap backend for key-value storage.                                                                                                 |
| [`fs`](pkg/store/backends/fs)                             | Filesystem backend for persistent storage.                                                                                                          |
| [`http`](pkg/store/backends/http)                         | HTTP backend for remote key-value storage.                                                                                                          |
| [`client`](pkg/store/backends/http/client)                | HTTP backend client implementation.                                                                                                                 |
| [`server`](pkg/store/backends/http/server)                | HTTP backend server implementation.                                                                                                                 |
| [`kubernetes`](pkg/store/backends/kubernetes)             | Kubernetes resource backend for key-value storage.                                                                                                  |
| [`labels`](pkg/store/backends/labels)                     | Labels backend for key-value storage.                                                                                                               |
| [`memory`](pkg/store/backends/memory)                     | In-memory backend for fast, ephemeral storage.                                                                                                      |
| [`replicate`](pkg/store/backends/replicate)               | Replication backend for high-availability storage.                                                                                                  |
| [`s3`](pkg/store/backends/s3)                             | S3 backend for object storage.                                                                                                                      |
| [`middleware`](pkg/store/middleware)                      | Store middleware for caching, logging, metrics, and more.                                                                                           |
| [`cache`](pkg/store/middleware/cache)                     | Cache middleware for stores.                                                                                                                        |
| [`logger`](pkg/store/middleware/logger)                   | Logger middleware for stores.                                                                                                                       |
| [`metrics`](pkg/store/middleware/metrics)                 | Metrics middleware for stores.                                                                                                                      |
| [`nameable`](pkg/store/middleware/nameable)               | Nameable middleware for stores.                                                                                                                     |
| [`prefix`](pkg/store/middleware/prefix)                   | Prefix middleware for stores.                                                                                                                       |
| [`threadsafe`](pkg/store/middleware/threadsafe)           | Threadsafe middleware for stores.                                                                                                                   |
| [`validatable`](pkg/store/middleware/validatable)         | Validatable middleware for stores.                                                                                                                  |
| [`utilities`](pkg/store/utilities)                        | Store utilities for advanced use cases.                                                                                                             |
| [`claimlock`](pkg/store/utilities/claimlock)              | Claim lock utility for distributed locking.                                                                                                         |
| [`defaultmap`](pkg/store/utilities/defaultmap)            | DefaultMap utility for stores.                                                                                                                      |
| [`timed`](pkg/store/utilities/timed)                      | Timed utility for time-based operations.                                                                                                            |
| [`task`](pkg/task)                                        | Workflow and job execution primitives for background and concurrent tasks, with support for chaining and retries.                                   |
| [`job`](pkg/task/job)                                     | Job execution primitives for background processing.                                                                                                 |
| [`runnable`](pkg/task/runnable)                           | Runnable task primitives for concurrent execution.                                                                                                  |
| [`workflow`](pkg/task/workflow)                           | Workflow execution primitives for chaining activities.                                                                                              |
| [`tlsclient`](pkg/tlsclient)                              | Utilities for creating and configuring TLS client configurations (mTLS, custom CAs, etc.), with certificate pool management.                        |
| [`tlsserver`](pkg/tlsserver)                              | Utilities for creating and configuring TLS server configurations (mTLS, client auth, etc.), with flexible validation.                               |
| [`tlsutils`](pkg/tlsutils)                                | TLS utility helpers for certificate and config management.                                                                                          |
| [`trace`](pkg/trace)                                      | Experimental: OpenTelemetry integration for distributed tracing, log correlation, and Gin instrumentation.                                          |
| [`utils`](pkg/utils)                                      | Miscellaneous utility functions for context, networking, randomness, and signals.                                                                   |

## The access and middleware auth packages
The `access` and `middleware/authorization` packages provide utilities for implementing fine-grained access control in
your applications using **scopes**. For this open source release we have tried to make the scopes as generic as possible,
but configuring them, e.g. via ginserver config, has room for improvements. We are working on improving the configuration 
experience and documentation for these packages, but in the meantime feel free to explore the code and reach out if you
have any questions.

## Orchestration Package

The orchestration package was the first initial open source offering. Below are details that were in the original readme.
The `pkg/orchestration` package is designed for concurrent synchronous `Service` calls. The package goes through several
stages for each `Service`:

- **Check**: Sanity check whether the `Service` request is likely to succeed
- (**Recover**: Advanced usage to recover from a failing Check, used to recover from corrupted/illegal states)
- **Run**: Executes the `Service` request. All `Service`s must have passed their `Check` stage.
- (**Rollback**: Is called for every `Service` when one or more `Service` has failed their `Run` stage)

### Quick Start

```go
// Define your Service
var _ Service = &MyService{}   // MyService implements Service
type MyService struct {
    orchestration.Recoverable, // Implements the Recover func to satisfy Service interface
    orchestration.Payload,     // Implements the GenerateResponse func to satisfy Service interface
    Datacenter string          // For example, to generate some complexity
}

func (svc *MyService) Name() string { return "MyService" }
func (svc *MyService) Check(_ context.Context) error { ... }
func (svc *MyService) Run(_ context.Context) error { ... }
func (svc *MyService) Rollback(_ context.Context) error { ... }

func main() {
    // Define the Services that you want to execute concurrently
    services := []Service{ &MyService{Datacenter: "DC1"}, &MyService{Datacenter: "DC2"}}
    
    // Call Check, Run, Rollback stages for each Service
    // errs align with services, err[i] corresponds with service[i]. err[i] may be nil
    // err is nil only when all errs are nil. Err contains information about which stage failed.
    errs, err := CallServices(context.TODO(), services)
    
    httpStatusCode, response := GenerateResponse(services, errs, err)
    // 200, {"status":"ok","details":[{"name":"MyService","detail":"<your-response>"}}
}
```

In the example above the services are executed in the following timeline:

```text
-> | MyService{DC1}.Check | -> | MyService{DC1}.Run | -> Rollback Skipped
   | MyService{DC2}.Check |    | MyService{DC2}.Run |
```

### Service Interface

The `Service` interface can be found in `pkg/orchestration/api_service.go`:

```go
type Service interface {
    Name() string
    
    Check(ctx context.Context) error
    Recover(ctx context.Context) error // Advanced usage, should return an error if not implemented
    Run(ctx context.Context) error
    Rollback(ctx context.Context) error
    
    GetResponse(err error) interface{}
}
```

### Service Response Patterns

Since the `Service` interface has no output, apart from the error, the output must be generated via your own
implementation. To help with this there is a `Payload` struct which has a response interface:

```go
struct MyService {
    Payload
}

func (svc *MyService) Run(_ context.Context) error {
    svc.Payload.Response = "Good!"
    return nil
}

func main() {
    services := []Services{ &MyService{} }
    errs, err := CallServices(context.TODO(), services)
    
    httpStatusCode, response := GenerateResponse(services, errs, err)
    
    // response: 200: {"status":"ok","details":[{"name":"MyService","detail":"Good!"}}
    // or, on check stage error: 500: {"status":"one or more pre-run checks failed","details":[{"name":"MyService","detail":"some-error"}}
    // or, on run stage error: 500: {"status":"one or more runs failed","details":[{"name":"MyService","detail":"some-error"}}
}
```

### Service Request Patterns

A request payload should be contained in your `Service` implementation. Consider two types of payloads:

1. A `Service` local payload - copied from the user's request, can be read/modified freely
2. A mutable payload shared across `Service`s (with an atomic lock)

```go
type MyService struct {
    LocalRequest MyServiceRequest // Can be read/modified freely
    
    mutex.Lock // Owns SharedRequest
    SharedRequest *MyServiceRequest // Shared state across Services, protected by Lock.
}
```

### Recovery

In some scenarios it may be possible to recover from a failing `Check`. Recovery is only executed by `CallServices` when `recover` is set in the `context`:

```go
type MyUpdateService struct {
    Recoverable // Provides a MyUpdateService.Recovery function pointer
    Request Spec
}

func (svc *MyUpdateService) Check(_ context.Context) error {
    if !database.Has(svc.Request.Name) {
        svc.Recovery = func(ctx context.Context) error {
            return MyCreateService{Request: svc.Request}.Run(ctx)
        }
        return errors.new("cannot update " + svc.Request.Name + " because it is not found")
    }
    return nil
}
```

### Rollback

When a `Service` has a rollback it is executed "in the background". Use the `RollbackErrorReporter` to handle rollback errors:

```go
orchestration.RollbackErrorReporter = func(services []orchestration.Service, errs []error) {
    for i := 0; i < len(services); i++ {
        if errs[i] != nil {
            log.Printf("Rollback failed for Service %s: %v", services[i].Name(), errs[i])
        }
    }
}
```

### Dry Runs

When the dryRun flag is specified in the `Context`, `CallServices` only executes the `Check` stage:

```go
ctx := context.WithValue(context.Background(), "dryRun", true)
_, _ = CallServices(ctx, []Services{ ... }) // Only calls Check stage for each Service
```

### REST API to Service Conversion

REST APIs can be generically converted to a `Service` interface:

| Operation | Check | Run | Rollback |
|-----------|-------|-----|----------|
| **GET** (Read/List) | None | Executes Get() | None |
| **POST** (Create) | Executes Get(), expects error | Executes Post() | Executes Delete() |
| **PUT** (Update) | Executes Get(), stores as `backup` | Executes Put() | Executes Put() with `backup` |
| **DELETE** | Executes Get(), stores as `backup` | Executes Delete() | Executes Create() with `backup` |

```go
var request Nameable = &SomePostedPayload{}
name, err := request.Name()

var exampleApi RestApi = &MyExampleApi{}
svc := RestApiAsService(exampleApi, "Example Create", name, request)

errs, err := CallServices(context.TODO(), []Service{svc})
```

### Staged Service Calls

When a `Service` depends on the output of another, use staged execution:

```go
sharedState := &Example{mutex.Lock{}, State: SomeState}
stageNum, errs, err := CallStagedServices(context.TODO(), [][]Service{
    {   // Stage 1
        &DependencyServiceA{SharedState: sharedState},
        &DependencyServiceB{SharedState: sharedState},
        MakeDryRun(&OtherServiceA{SharedState: sharedState}),
        MakeDryRun(&OtherServiceB{SharedState: sharedState}),
    },
    {   // Stage 2 - Executed after all Run's of stage 1 were successful
        &OtherServiceA{SharedState: sharedState},
        &OtherServiceB{SharedState: sharedState},
    },
})
```

## Example Applications

This repository includes two example applications demonstrating the Create Memory Claim service:

- **Service implementation**: Fine-grained control over Check/Run/Rollback stages
- **RestApi implementation**: Automatically transformed to a Service with less code

### Example Usage

```bash
# Successful create
$ curl http://localhost:8090/api/v1/memory -d @example-payloads/payload_ok.json
{"status":"ok","details":[
  {"name":"MyService Create DC1_BLUE","detail":"ok"},
  {"name":"MyService Create DC1_RED","detail":"ok"},
  {"name":"MyService Create DC2_BLUE","detail":"ok"},
  {"name":"MyService Create DC2_RED","detail":"ok"}
]}

# Duplicate create (Check stage catches conflict)
$ curl http://localhost:8090/api/v1/memory -d @example-payloads/payload_ok.json
{"status":"one or more pre-run checks failed","details":[
  {"name":"MyService Create DC1_BLUE","detail":"already exists"},
  ...
]}

# Failed create with automatic Rollback
$ curl http://localhost:8090/api/v1/memory -d @example-payloads/payload_fail.json
{"status":"one or more runs failed","details":[
  {"name":"MyService Create DC1_BLUE","detail":"ok"},
  {"name":"MyService Create DC1_RED","detail":"not enough memory available"},
  ...
]}
```

## Contributing

Contributions are welcome! Please open issues or pull requests. See [`CONTRIBUTING.md`](CONTRIBUTING.md) for guidelines.
