## PDP Structure

```
pdp/
    ├── model/
    │   ├── request.go         # Defines the AccessRequest struct and related types
    │   └── decision.go        # Defines the AccessDecision struct and related types
    ├── engine/
    │   ├── evaluator.go       # Core policy evaluation logic
    │   ├── combiner.go        # Implements decision combining algorithms
    │   └── cache.go           # Caching mechanisms for policy and attribute data
    ├── resolver/
    │   ├── attribute.go       # Attribute resolution logic
    │   └── policy.go          # Policy retrieval and resolution
    ├── service/
    │   └── pdp_service.go     # Main PDP service implementation
    ├── config/
    │   └── pdp_config.go      # Configuration settings for PDP
    ├── util/
    │   ├── logger.go          # PDP-specific logging utilities
    │   └── error.go           # PDP-specific error types and handling
    ├── middleware/
    │   └── pdp_middleware.go  # Middleware for integrating PDP with HTTP handlers
    └── test/
        ├── unit/              # Unit tests for individual components
        │   ├── evaluator_test.go
        │   ├── combiner_test.go
        │   └── resolver_test.go
        └── integration/       # Integration tests for the PDP service
            └── pdp_service_test.go
```

## ToDos

- Access Request Model:
  - Define a structure to represent an access request, including subject (user), resource, action, and environment attributes.- Policy Retrieval:
  - Implement a mechanism to efficiently retrieve relevant policies based on the access request.- Attribute Resolution:
  - Develop a system to resolve and fetch all necessary attributes for the subject, resource, action, and environment.- Policy Evaluation Engine:
  - Create the core logic to evaluate policies against the access request and resolved attributes.
  - This should include handling different types of conditions and rules within policies.- Decision Combining Algorithm:
  - Implement logic to combine decisions from multiple applicable policies (e.g., deny-overrides, permit-overrides).- Caching Mechanism:
  - Design a caching strategy for frequently accessed policies and attribute values to improve performance.- Decision Logging:
  - Implement comprehensive logging of access decisions for auditing and troubleshooting.- Error Handling and Edge Cases:
  - Plan for handling various error scenarios and edge cases in the decision-making process.- Performance Optimization:
  - Consider strategies to optimize the evaluation process, especially for systems with a large number of policies.- Integration Points:
  - Define how the PDP will integrate with your existing services and controllers.- Testing Framework:
  - Design a robust testing strategy for the PDP, including unit tests and integration tests.- Extensibility:
  - Plan for future extensions, such as supporting new types of attributes or policy languages.
