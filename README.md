# Storr API

A serverless authentication, store management, and integration API built with Go, deployed on AWS Lambda.

## Tech Stack

- **Language:** Go 1.25
- **Router:** [Chi](https://github.com/go-chi/chi) v5
- **Auth:** AWS Cognito (JWT with JWKS)
- **Database:** AWS DynamoDB
- **Container Orchestration:** AWS ECS Fargate
- **Load Balancing:** AWS ALB (preview routes)
- **Secrets:** AWS Secrets Manager
- **Parameters:** AWS Systems Manager (SSM)
- **Runtime:** AWS Lambda + API Gateway
- **Integrations:** Figma OAuth, Shopify OAuth

## Architecture

The API follows a **Handler → Service → Repository** layering pattern:

- **Handlers** — HTTP request/response, validation, error mapping
- **Services** — Business logic, orchestration
- **Repositories** — DynamoDB operations (via AWS SDK)

Cross-cutting concerns (auth middleware, response formatting) live in shared packages.

## Project Structure

```
storr-api/
├── cmd/
│   └── storr-api/
│       ├── main.go              # Lambda entry point
│       ├── config.go            # Environment variable loading
│       ├── deps.go              # Dependency construction
│       └── router.go            # Route registration
└── internal/
    ├── api/
    │   └── response.go          # Typed HTTP response helpers
    ├── auth/
    │   ├── handler.go           # Auth HTTP handlers
    │   ├── service.go           # Cognito auth logic + JWT verification
    │   ├── middleware.go         # JWT auth middleware
    │   ├── token.go             # Bearer token extraction
    │   ├── context.go           # Request context utilities
    │   └── model.go             # Auth data models
    ├── store/
    │   ├── handler.go           # Store HTTP handlers
    │   ├── service.go           # DynamoDB store logic
    │   └── model.go             # Store data models
    ├── studio/
    │   ├── handler.go           # Studio HTTP handlers
    │   ├── service.go           # ECS/ALB orchestration
    │   ├── model.go             # Studio data models + payload structs
    │   ├── task_helpers.go      # Task lookup, ENI extraction, ownership
    │   └── preview_route.go     # ALB target group & listener rule management
    ├── infra/
    │   ├── aws_clients.go       # AWSClients facade (ECS, EC2, ELBV2)
    │   ├── runtime_client.go    # Runtime client (status, theme, input)
    │   └── ecs_helpers.go       # ECS task helper utilities
    ├── pipeline/
    │   ├── handler.go           # Pipeline HTTP handlers
    │   ├── service.go           # Pipeline orchestration logic
    │   ├── theme_service.go     # Theme selection service
    │   ├── theme_config.go      # Theme task configuration
    │   ├── task_resolver.go     # ECS task resolution
    │   └── model.go             # Pipeline data models
    └── integration/
        ├── handler.go           # Integration HTTP handlers
        ├── service.go           # DynamoDB integration records
        ├── figma.go             # Figma OAuth flow
        ├── shopify.go           # Shopify OAuth flow + HMAC + GraphQL
        ├── provisioner.go       # Build ECS env overrides from integrations
        ├── readiness.go         # Integration readiness checks
        ├── callback.go          # OAuth callback handler
        ├── oauth_state.go       # DynamoDB OAuth state store
        ├── secrets.go           # Generic Secrets Manager client
        └── model.go             # Integration data models
```

## Prerequisites

- Go 1.25+
- AWS credentials configured (`aws configure`)
- AWS Cognito User Pool + App Client
- DynamoDB tables for stores and integrations
- ECS cluster with studio task definition
- ALB with listener for preview routes

## Environment Variables

| Variable                    | Description                                          |
| --------------------------- | ---------------------------------------------------- |
| `COGNITO_USER_POOL_ID`      | AWS Cognito User Pool ID                             |
| `COGNITO_CLIENT_ID`         | Cognito App Client ID                                |
| `CORS_ORIGIN`               | Allowed CORS origin                                  |
| `STORES_TABLE`              | DynamoDB table for stores                            |
| `INTEGRATIONS_TABLE`        | DynamoDB table for integrations                      |
| `STUDIO_CLUSTER`            | ECS cluster name                                     |
| `STUDIO_TASK_DEFINITION`    | ECS task definition ARN                              |
| `STUDIO_SUBNETS`            | Comma-separated subnet IDs                           |
| `STUDIO_SECURITY_GROUP`     | Security group ID                                    |
| `STUDIO_ALB_LISTENER_ARN`   | ALB listener ARN for preview routes                  |
| `STUDIO_VPC_ID`             | VPC ID for target groups                             |
| `PIPELINE_CLUSTER`          | ECS cluster for pipeline tasks (falls back to `STUDIO_CLUSTER`) |
| `PIPELINE_TASK_DEFINITION`  | ECS task definition for pipeline                     |
| `THEME_CLUSTER`             | ECS cluster for theme tasks (falls back to `STUDIO_CLUSTER`)    |
| `THEME_TASK_DEFINITION`     | ECS task definition for theme                        |
| `THEME_CONTAINER_NAME`      | Container name for theme tasks (defaults to `studio-env`)       |
| `ANTHROPIC_API_KEY_PARAM`   | SSM parameter for Anthropic API key                  |
| `FIGMA_CLIENT_ID`           | Figma OAuth client ID                                |
| `FIGMA_CLIENT_SECRET`       | Figma OAuth client secret                            |
| `FIGMA_REDIRECT_URI`        | Figma OAuth redirect URI (points to storr-api `/oauth/figma/callback`) |
| `SHOPIFY_CLIENT_ID`         | Shopify OAuth client ID                              |
| `SHOPIFY_CLIENT_SECRET`     | Shopify OAuth client secret                          |
| `SHOPIFY_REDIRECT_URI`      | Shopify OAuth redirect URI (points to storr-api `/oauth/shopify/callback`) |
| `OAUTH_STATES_TABLE`        | DynamoDB table for OAuth state                       |
| `FRONTEND_URL`              | Frontend URL for OAuth callback redirects            |
| `RUNTIME_AUTH_TOKEN_PARAM`  | SSM parameter path for runtime auth token            |

## Build

```bash
go build -o storr-api ./cmd/storr-api
```

## API Endpoints

### Auth (No Auth Required)

| Method | Path            | Description            |
| ------ | --------------- | ---------------------- |
| POST   | `/auth/signup`  | Register a new user    |
| POST   | `/auth/login`   | Login                  |
| POST   | `/auth/logout`  | Clear session cookies  |
| POST   | `/auth/refresh` | Refresh access token   |
| GET    | `/auth/me`      | Get current user       |

### Stores (Authenticated)

| Method | Path            | Description         |
| ------ | --------------- | ------------------- |
| GET    | `/stores`       | List user's stores  |
| POST   | `/stores`       | Create a store      |
| GET    | `/stores/{id}`  | Get a store         |
| PUT    | `/stores/{id}`  | Update a store      |
| DELETE | `/stores/{id}`  | Delete a store      |

### Pipeline (Authenticated, nested under `/stores/{id}`)

| Method | Path                              | Description                   |
| ------ | --------------------------------- | ----------------------------- |
| POST   | `/stores/{id}/provision`          | Provision pipeline env        |
| GET    | `/stores/{id}/status`             | Get pipeline status           |
| GET    | `/stores/{id}/readiness`          | Check pipeline readiness      |
| POST   | `/stores/{id}/select-theme`       | Select a Shopify theme        |
| POST   | `/stores/{id}/provide-input`      | Provide input to runtime      |

### Studio (Authenticated)

| Method | Path                     | Description                   |
| ------ | ------------------------ | ----------------------------- |
| POST   | `/studio/provision`      | Provision ECS environment     |
| GET    | `/studio/status`         | Get environment status        |
| POST   | `/studio/chat`           | Send a message                |
| POST   | `/studio/select-theme`   | Select a Shopify theme        |
| POST   | `/studio/provide-input`  | Provide input to runtime      |

### Integrations (Authenticated)

| Method | Path                                                | Description               |
| ------ | --------------------------------------------------- | ------------------------- |
| GET    | `/customers/{id}/integrations`                      | List integrations         |
| POST   | `/customers/{id}/integrations/figma/start`          | Start Figma OAuth         |
| POST   | `/stores/{id}/integrations/shopify/start`           | Start Shopify OAuth       |

### OAuth Callbacks (No Auth Required)

| Method | Path                     | Description                        |
| ------ | ------------------------ | ---------------------------------- |
| GET    | `/oauth/figma/callback`  | Figma OAuth callback               |
| GET    | `/oauth/shopify/callback`| Shopify OAuth callback             |

## Coding Constraints

1. No function body exceeds 20 lines
2. No function has more than 4 parameters
3. No parameter struct has more than 4 fields
4. No use of `any` or `interface{}`
5. All HTTP responses use typed structs (no ad-hoc maps)
6. All errors are sanitized before sending to clients
7. All JWT claims use typed structs (no `MapClaims`)

## Security

- **Typed JWT Verification**: Hardened JWT claim verification using `cognitoClaims` struct with `ParseWithClaims`. Validates issuer, audience/client_id, and expiration against Cognito JWKS.
- **HttpOnly Cookies**: Authentication tokens stored in `HttpOnly`, `Secure`, `SameSite=None` cookies.
- **CORS**: Strict origin validation with credentials support.
- **HMAC Validation**: Shopify OAuth callbacks verified with HMAC-SHA256 (validates all query params per Shopify spec).
- **Error Sanitization**: Internal errors are logged server-side; only generic messages reach the client.
- **Secrets Management**: OAuth tokens stored in AWS Secrets Manager with automatic refresh.

## Deployment

The API is designed to run as an AWS Lambda function behind API Gateway. Package the compiled binary as a ZIP and deploy to Lambda.
