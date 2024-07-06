# Project Structure

```bash
/ark
│
├── /api                  # Golang backend
│   ├── /controller       # Request handlers
│   ├── /service          # Business logic
│   ├── /dao              # Data access objects for Neo4j
│   ├── /model            # Data models
│   ├── /config           # Configuration files and environment variables
│   ├── /middleware       # Middleware for authentication, rate limiting
│   ├── /util             # Utilities and helper functions
│   └── main.go           # Main application entry point
│
├── /client               # Next.js frontend
│   ├── /pages            # All the React components for each page
│   ├── /components       # Reusable React components
│   ├── /styles           # CSS/SCSS files
│   ├── /public           # Static files like images, fonts, etc.
│   └── /lib              # Libraries and hooks
│
├── /neo4j                # Neo4j specific configurations
│   ├── /migrations       # Neo4j schema migrations
│   └── /seeds            # Seed data for initial setup
│
├── /redis                # Redis configuration files
│   └── /setup            # Scripts for setting up caching patterns and rate limits
│
├── /scripts              # Utility scripts for deployment, local development
│
├── /deploy               # Deployment configurations (Dockerfiles, docker-compose, k8s)
│   ├── Dockerfile        # Dockerfile for the backend
│   ├── Dockerfile.front  # Dockerfile for the frontend
│   └── docker-compose.yml# Compose file to orchestrate services
│
└── README.md             # Project overview and setup instructions
```
