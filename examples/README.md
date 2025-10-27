# stackver Examples

This directory contains example configurations showing how to use stackver with different deployment patterns.

## Examples

### [kubernetes/](kubernetes/)
Kubernetes deployment with multiple containers:
- **deployment.yaml** - Kubernetes deployment with nginx and redis containers
- **stack.yaml** - stackver manifest tracking both container images

```bash
cd examples/kubernetes
stackver -f stack.yaml
stackver -f stack.yaml -dry-run
stackver -f stack.yaml -update
```

### [helm/](helm/)
Helm chart values with multiple image tags:
- **values.yaml** - Helm values for cert-manager with multiple components
- **stack.yaml** - stackver manifest tracking all cert-manager images

```bash
cd examples/helm
stackver -f stack.yaml
stackver -f stack.yaml -update
```

### [docker-compose/](docker-compose/)
Docker Compose with multiple services:
- **docker-compose.yml** - Multi-service application stack
- **stack.yaml** - stackver manifest tracking all service images

```bash
cd examples/docker-compose
stackver -f stack.yaml
stackver -f stack.yaml -update
```

## Testing

Each example includes realistic version numbers that are likely to have updates available, so you can test the full stackver workflow:

1. **Check status** - See current vs latest versions
2. **Preview updates** - Use `-dry-run` to see what would change
3. **Apply updates** - Use `-update` to modify files

## JSONPath Selectors

The examples demonstrate common JSONPath patterns:

- `$.spec.template.spec.containers[0].image` - Kubernetes container image
- `$.image.tag` - Helm image tag
- `$.services.web.image` - Docker Compose service image

## Trackers

Examples use both supported trackers:

- **endoflife** - For tracking EOL dates (nginx, redis, postgresql, nodejs)
- **github** - For GitHub releases (cert-manager)
