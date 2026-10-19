**Online Boutique** is a containerized microservices demo application. The application is a
web-based e-commerce app where users can browse items, add them to the cart, and purchase them.

This repository focuses on running the complete application locally with Docker Compose. It demonstrates service-to-service communication with [gRPC](https://grpc.io/), a polyglot service architecture, container builds, and a Redis-backed shopping cart without requiring an external platform account.

## Architecture

**Online Boutique** contains 11 services written in different languages that talk to each other over gRPC. The default Docker Compose stack runs the storefront services and Redis; the load generator remains available in the source tree but is not started by default.

[![Architecture of
microservices](/docs/img/architecture-diagram.png)](/docs/img/architecture-diagram.png)

Find **Protocol Buffers Descriptions** at the [`./protos` directory](/protos).

| Service                                              | Language      | Description                                                                                                                       |
| ---------------------------------------------------- | ------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| [frontend](/src/frontend)                           | Go            | Exposes an HTTP server to serve the website. Does not require signup/login and generates session IDs for all users automatically. |
| [cartservice](/src/cartservice)                     | C#            | Stores the items in the user's shopping cart in Redis and retrieves it.                                                           |
| [productcatalogservice](/src/productcatalogservice) | Go            | Provides the list of products from a JSON file and ability to search products and get individual products.                        |
| [currencyservice](/src/currencyservice)             | Node.js       | Converts one money amount to another currency. Uses real values fetched from European Central Bank. It's the highest QPS service. |
| [paymentservice](/src/paymentservice)               | Node.js       | Charges the given credit card info (mock) with the given amount and returns a transaction ID.                                     |
| [shippingservice](/src/shippingservice)             | Go            | Gives shipping cost estimates based on the shopping cart. Ships items to the given address (mock)                                 |
| [emailservice](/src/emailservice)                   | Python        | Sends users an order confirmation email (mock).                                                                                   |
| [checkoutservice](/src/checkoutservice)             | Go            | Retrieves user cart, prepares order and orchestrates the payment, shipping and the email notification.                            |
| [recommendationservice](/src/recommendationservice) | Python        | Recommends other products based on what's given in the cart.                                                                      |
| [adservice](/src/adservice)                         | Java          | Provides text ads based on given context words.                                                                                   |
| [loadgenerator](/src/loadgenerator)                 | Python/Locust | Continuously sends requests imitating realistic user shopping flows to the frontend.                                              |

## Screenshots

| Home Page                                                                                                         | Checkout Screen                                                                                                    |
| ----------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------ |
| [![Screenshot of store homepage](/docs/img/online-boutique-frontend-1.png)](/docs/img/online-boutique-frontend-1.png) | [![Screenshot of checkout screen](/docs/img/online-boutique-frontend-2.png)](/docs/img/online-boutique-frontend-2.png) |

## Quickstart (Docker Compose)

1. Ensure you have the following requirements:
   - [Docker Desktop](https://www.docker.com/products/docker-desktop/) or Docker Engine with the Compose plugin.

2. From the repository root, build the service images and start the application in the background.

   ```sh
   docker compose up --build -d
   ```

   The first build may take a few minutes while Docker downloads base images and service dependencies.

3. Check that the containers are running.

   ```sh
   docker compose ps
   ```

4. Visit [http://localhost:8080](http://localhost:8080) in a web browser to access Online Boutique.

5. Follow the application logs when troubleshooting or developing a service.

   ```sh
   docker compose logs -f
   ```

6. Stop and remove the application containers when you are done.

   ```sh
   docker compose down
   ```

## Common development commands

- Rebuild one service: `docker compose build <service-name>`
- Rebuild and restart one service: `docker compose up -d --build <service-name>`
- Follow selected logs: `docker compose logs -f frontend checkoutservice`
- Stop the stack and remove its containers: `docker compose down`

## Documentation

- [Adding a new microservice](/docs/adding-new-microservice.md)
- [Product requirements](/docs/product-requirements.md)
- [Project purpose](/docs/purpose.md)
