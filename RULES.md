# Hooklab Rule Engine

The rule engine allows you to configure conditional responses based on incoming request data. Rules are evaluated in priority order, and the first matching rule determines the response.

## Quick Start

1. Go to the Rules page: Click "⚡ Rules" on the main monitor page
2. Create a rule with a condition expression
3. Set the response body and status code
4. Enable the rule

## Available Variables

| Variable | Type | Description |
|----------|------|-------------|
| `body` | `map` or `string` | Parsed JSON body, or raw string if not valid JSON |
| `method` | `string` | HTTP method: `GET`, `POST`, `PUT`, `DELETE`, etc. |
| `headers` | `map[string][]string` | Request headers |

## Expression Syntax

Hooklab uses the [expr](https://github.com/expr-lang/expr) expression language. Here's a comprehensive guide:

### Comparison Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `==` | Equal | `body.status == "active"` |
| `!=` | Not equal | `body.type != "test"` |
| `<` | Less than | `body.amount < 100` |
| `<=` | Less than or equal | `body.count <= 10` |
| `>` | Greater than | `body.price > 50` |
| `>=` | Greater than or equal | `body.quantity >= 1` |

### Logical Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `&&` | AND | `body.amount > 100 && body.currency == "USD"` |
| `\|\|` | OR | `body.status == "error" \|\| body.status == "failed"` |
| `!` | NOT | `!body.verified` |

### String Operations

```
body.name == "John"                    // Exact match
body.email contains "@gmail.com"       // Contains substring
body.name startsWith "Dr."             // Starts with
body.url endsWith ".pdf"               // Ends with
body.name matches "^[A-Z].*"           // Regex match
```

### Membership & Existence

```
// Check if key exists in map
"email" in body

// Check if value is in array
body.status in ["pending", "processing", "completed"]

// Check if array contains value
"admin" in body.roles
```

### Numeric Operations

```
body.amount + body.tax > 100           // Addition
body.price * body.quantity >= 500      // Multiplication
body.total / body.count < 50           // Division
body.value % 2 == 0                    // Modulo (even number)
```

### Array Operations

```
len(body.items) > 0                    // Array length
len(body.items) == 5                   // Exact count
body.items[0].type == "premium"        // Access by index
```

### Nil/Null Checks

```
body.optional == nil                   // Check if null
body.optional != nil                   // Check if not null
body.field ?? "default"                // Nil coalescing
```

### Ternary Operator

```
body.amount > 100 ? "high" : "low"     // Conditional value
```

### Working with Headers

Headers are stored as `map[string][]string` (each header can have multiple values):

```
// Check if header exists
"Authorization" in headers
"Content-Type" in headers

// Access header value (first value)
headers["Content-Type"][0] == "application/json"
headers["X-Api-Key"][0] startsWith "sk_"
```

## Example Rules

### 1. High-Value Payment Detection

**Condition:**
```
body.type == "payment" && body.amount > 1000
```

**Response:**
```json
{
  "status": "review_required",
  "message": "High-value transaction flagged for review"
}
```

### 2. Error Simulation for Testing

**Condition:**
```
body.simulate_error == true
```

**Response (Status 500):**
```json
{
  "error": "Internal Server Error",
  "code": "SIMULATED_ERROR"
}
```

### 3. Method-Based Routing

**Condition:**
```
method == "DELETE"
```

**Response (Status 403):**
```json
{
  "error": "Delete operations are not allowed"
}
```

### 4. API Version Check

**Condition:**
```
"X-Api-Version" in headers && headers["X-Api-Version"][0] == "v2"
```

**Response:**
```json
{
  "api_version": "v2",
  "features": ["enhanced", "beta"]
}
```

### 5. Webhook Type Routing

**Condition:**
```
body.event in ["order.created", "order.updated", "order.completed"]
```

**Response:**
```json
{
  "received": true,
  "queue": "orders"
}
```

### 6. Complex Business Logic

**Condition:**
```
body.user.tier == "premium" && body.order.total > 500 && len(body.order.items) >= 3
```

**Response:**
```json
{
  "discount_applied": true,
  "discount_percent": 15,
  "message": "Premium bulk discount applied"
}
```

## Rule Priority

- Rules are evaluated in **priority order** (lower number = higher priority)
- **First matching rule wins** — subsequent rules are not evaluated
- If no rule matches, the **default response** is returned

| Priority | Rule Name | Condition |
|----------|-----------|-----------|
| 0 | Critical Error | `body.critical == true` |
| 1 | High Value | `body.amount > 1000` |
| 10 | Default Success | `true` |

## Tips

1. **Start specific, end general**: Put specific rules at lower priority numbers
2. **Use `true` as a catch-all**: A rule with condition `true` always matches
3. **Test expressions**: Invalid expressions are skipped silently during evaluation
4. **JSON body required**: For `body.field` access, the request must have valid JSON

## API Reference

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/rules?key={key}` | List all rules for a webhook key |
| `POST` | `/api/rules?key={key}` | Create a new rule |
| `PUT` | `/api/rules?key={key}&id={id}` | Update an existing rule |
| `DELETE` | `/api/rules?key={key}&id={id}` | Delete a rule |

### Create Rule Request

```bash
curl -X POST "http://localhost:8080/api/rules?key=payments" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "High Value Alert",
    "condition": "body.amount > 1000",
    "response": {"status": "flagged"},
    "statusCode": 200,
    "priority": 1,
    "enabled": true
  }'
```

## Further Reading

- [expr Language Definition](https://expr-lang.org/docs/language-definition)
- [expr Operator Reference](https://expr-lang.org/docs/operators)
- [expr Built-in Functions](https://expr-lang.org/docs/builtin-functions)
