# Data Flow

## Overview

This document outlines the data flow between different components of the DevLM system.

## Request Flow

1. Client Request -> API Gateway
2. API Gateway -> Golang Backend Service
3. Golang Backend Service -> Python LLM Service
4. Python LLM Service -> Claude Sonnet Instances
5. Claude Sonnet Instances -> Python LLM Service
6. Python LLM Service -> Golang Backend Service
7. Golang Backend Service -> Code Execution Engine
8. Code Execution Engine -> Golang Backend Service
9. Golang Backend Service -> Action Executor
10. Action Executor -> Golang Backend Service
11. Golang Backend Service -> API Gateway
12. API Gateway -> Client Response

## Component Interactions

### Client Request to API Gateway