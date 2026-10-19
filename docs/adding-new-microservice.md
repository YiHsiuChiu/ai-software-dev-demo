# Adding a new microservice

This document outlines the steps required to add a new microservice to the Online Boutique application.

## 1. Create a new directory

Create a new directory for your microservice within the `src/` directory. The directory name should be the name of your microservice.

## 2. Add source code

Place your microservice's source code inside the newly created directory. The structure of this directory should follow the conventions of the existing microservices. For example, a Python-based service would include at minimum the following files:

- `README.md`: The service's description and documentation.
- `main.py`: The application's entry point.
- `requirements.in`: A list of Python dependencies.
- `Dockerfile`: To containerize the application.

Take a look at existing microservices for inspiration.

## 3. Create a Dockerfile

Create a `Dockerfile` in your microservice's directory. This file will define the steps to build a container image for your service.

Refer to the [`frontend` Dockerfile](/src/frontend/Dockerfile) and tweak it based on your new service's needs.

## 4. Update `compose.yaml`

Add your new service under `services` in the root [`compose.yaml`](/compose.yaml). A service definition typically includes:

- A `build` path that points to the service's Docker build context.
- The environment variables required by the service.
- A `depends_on` list when the service relies on other containers.
- A host `ports` mapping only when the service must be accessed directly from the host.

Follow the existing naming conventions. Containers can reach the new service by using its Compose service name and container port, for example `myservice:8080`.

Refer to an existing service in [`compose.yaml`](/compose.yaml) and adjust it to match your service's build context, port, and dependencies.

## 5. Connect dependent services

If another service calls the new microservice, add the new endpoint to that service's environment variables in `compose.yaml`. Use the Compose service name rather than `localhost`, because each service runs in its own container.

If the service communicates over gRPC, update the Protocol Buffer definitions and generated clients as needed.

## 6. Build and verify the service

Validate the Compose configuration, then build and start the service from the repository root:

```sh
docker compose config
docker compose up -d --build <service-name>
docker compose logs -f <service-name>
```

## 7. Update the documentation

Finally, update the project's documentation to reflect the addition of your new microservice. This may include:

- Adding a section to the main `README.md` if the service introduces significant new functionality.
- Updating the architecture diagrams in the `docs/img` directory.
- Adding a new document in the `docs` directory if the service requires detailed explanation.
