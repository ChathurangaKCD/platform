# Common Expression Language (CEL) - Developer Guide

A practical guide to CEL expressions covering all built-in features, operators, and methods.

## Table of Contents

- [Overview](#overview)
- [Data Types](#data-types)
- [Operators](#operators)
- [String Operations](#string-operations)
- [List Operations](#list-operations)
- [Map Operations](#map-operations)
- [Type Conversion](#type-conversion)
- [Timestamp & Duration](#timestamp--duration)
- [Macros](#macros)
- [Advanced Features](#advanced-features)
- [Practical Examples](#practical-examples)

---

## Overview

CEL is a non-Turing complete expression language designed to be fast, portable, safe, and simple. It's used for expressing policies, validation rules, and conditional logic in a safe, sandboxed environment.

**Key Characteristics:**
- Memory-safe (no pointer arithmetic or buffer overflows)
- Side-effect free (expressions don't modify state)
- Terminating (all expressions complete in bounded time)
- C-like syntax for familiarity

---

## Data Types

### Primitive Types

| Type | Description | Examples |
|------|-------------|----------|
| `int` | 64-bit signed integers | `-42`, `100`, `0` |
| `uint` | 64-bit unsigned integers | `42u`, `100u` |
| `double` | 64-bit IEEE floating-point | `3.14`, `2.5e10`, `-0.5` |
| `bool` | Boolean values | `true`, `false` |
| `string` | Unicode strings | `"hello"`, `'world'` |
| `bytes` | Byte sequences | `b"hello"`, `b'\x00\xff'` |
| `null_type` | Null value | `null` |

### Aggregate Types

```cel
// Lists - ordered sequences
[1, 2, 3]
["apple", "banana", "cherry"]

// Maps - key-value collections
{"key": "value", "count": 42}
{"name": "John", "age": 30}
```

### Special Types

- **timestamp**: Date/time values (RFC3339 format)
- **duration**: Time intervals (`1h`, `30m45s`)
- **dyn**: Dynamic type for gradual typing

### String Literals

```cel
"double quotes"
'single quotes'
r"raw\nstring"          // Raw strings (no escape sequences)
"""multi-line
string"""
```

---

## Operators

Listed by precedence (highest to lowest):

### 1. Member Access & Indexing

```cel
person.name             // Field selection
list[0]                 // Index access
map["key"]              // Map access
```

### 2. Unary Operators

```cel
!true                   // Logical NOT → false
-5                      // Negation → -5
```

### 3. Multiplicative

```cel
3 * 4                   // Multiplication → 12
10 / 2                  // Division → 5
10 % 3                  // Modulo → 1
```

### 4. Additive

```cel
1 + 2                   // Addition → 3
"hi" + " there"         // Concatenation → "hi there"
[1, 2] + [3, 4]         // List concatenation → [1, 2, 3, 4]
10 - 3                  // Subtraction → 7
```

### 5. Relational

```cel
3 < 5                   // Less than → true
5 <= 5                  // Less than or equal → true
10 > 5                  // Greater than → true
5 >= 3                  // Greater than or equal → true
"a" in ["a", "b"]       // Membership → true
```

### 6. Equality

```cel
5 == 5                  // Equals → true
5 != 3                  // Not equals → true
```

### 7. Logical AND

```cel
true && true            // Conditional AND → true
false && true           // → false (short-circuits)
```

### 8. Logical OR

```cel
true || false           // Conditional OR → true
false || false          // → false (short-circuits)
```

### 9. Ternary Conditional

```cel
x > 0 ? "positive" : "non-positive"
age >= 18 ? "adult" : "minor"
```

---

## String Operations

### Basic Methods

```cel
// Length
"hello".size()                          // → 5
size("world")                           // → 5

// Substring
"hello world".substring(0, 5)           // → "hello"
"hello world".substring(6)              // → "world"

// Character access
"hello".charAt(0)                       // → "h"

// Case conversion
"Hello World".lowerAscii()              // → "hello world"
"hello world".upperAscii()              // → "HELLO WORLD"

// Trim whitespace
"  hello  ".trim()                      // → "hello"
```

### Pattern Matching & Searching

```cel
// Pattern matching (regex)
"hello123".matches('[a-z]+[0-9]+')     // → true
"user@example.com".matches(r'^\w+@\w+\.\w+$')  // → true

// Check prefix/suffix
"hello world".startsWith("hello")       // → true
"hello world".endsWith("world")         // → true

// Contains substring
"hello world".contains("lo wo")         // → true

// Find pattern
"abc 123".find('[0-9]+')                // → "123"

// Index of substring
"hello world".indexOf("o")              // → 4
"hello world".lastIndexOf("o")          // → 7
```

### String Manipulation

```cel
// Replace
"hello world".replace("world", "CEL")   // → "hello CEL"
"aaa".replace("a", "b", 2)              // → "bba" (limit 2)

// Split
"refs/heads/main".split('/')            // → ["refs", "heads", "main"]
"a,b,c".split(',')                      // → ["a", "b", "c"]

// Format (printf-style)
"Hello, %s! You are %d.".format(["World", 30])  // → "Hello, World! You are 30."
"%s: %.2f".format(["Price", 19.99])     // → "Price: 19.99"
```

---

## List Operations

### Basic Functions

```cel
// Size
[1, 2, 3].size()                        // → 3

// Indexing
[10, 20, 30][0]                         // → 10
[10, 20, 30][-1]                        // Last element (if supported)

// Slicing
[1, 2, 3, 4, 5].slice(1, 3)             // → [2, 3]

// Concatenation
[1, 2] + [3, 4]                         // → [1, 2, 3, 4]

// Membership
3 in [1, 2, 3, 4]                       // → true
10 in [1, 2, 3]                         // → false
```

### List Methods

```cel
// Search
[1, 2, 3, 2, 1].indexOf(2)              // → 1 (first occurrence)
[1, 2, 3, 2, 1].lastIndexOf(2)          // → 3 (last occurrence)

// Aggregation
[1, 5, 3, 9, 2].min()                   // → 1
[1, 5, 3, 9, 2].max()                   // → 9
[1, 2, 3, 4, 5].sum()                   // → 15

// Manipulation
[3, 1, 4, 1, 5].sort()                  // → [1, 1, 3, 4, 5]
[1, 2, 2, 3, 3, 4].distinct()           // → [1, 2, 3, 4]
[1, 2, 3].reverse()                     // → [3, 2, 1]
[[1, 2], [3, 4]].flatten()              // → [1, 2, 3, 4]

// Validation
[1, 2, 3, 4, 5].isSorted()              // → true
[3, 1, 2].isSorted()                    // → false
```

### List Macros

```cel
// filter - Select elements matching condition
[1, 2, 3, 4, 5].filter(x, x > 2)        // → [3, 4, 5]
users.filter(u, u.age >= 18)

// map - Transform each element
[1, 2, 3].map(x, x * 2)                 // → [2, 4, 6]
names.map(n, n.upperAscii())

// all - Check if all elements satisfy condition
[2, 4, 6, 8].all(x, x % 2 == 0)         // → true
scores.all(s, s >= 0 && s <= 100)

// exists - Check if any element satisfies condition
[1, 2, 3, 4].exists(x, x > 3)           // → true
emails.exists(e, e.endsWith("@example.com"))

// exists_one - Check if exactly one element satisfies
[1, 2, 3, 4].exists_one(x, x == 3)      // → true
items.exists_one(i, i.primary)
```

### Joining Lists

```cel
["a", "b", "c"].join("-")               // → "a-b-c"
["hello", "world"].join(" ")            // → "hello world"
```

---

## Map Operations

### Basic Functions

```cel
// Size
{"a": 1, "b": 2}.size()                 // → 2

// Access
{"name": "John", "age": 30}["name"]     // → "John"
{"name": "John", "age": 30}.name        // → "John"

// Membership (checks keys)
"name" in {"name": "John", "age": 30}   // → true
"email" in {"name": "John"}             // → false
```

### Map Methods

```cel
// Get all keys
{"a": 1, "b": 2, "c": 3}.keys()         // → ["a", "b", "c"]

// Get all values
{"a": 1, "b": 2, "c": 3}.values()       // → [1, 2, 3]
```

### Map Macros

```cel
// all - Check all entries
{"a": 1, "b": 2}.all(k, v, v > 0)       // → true
config.all(k, v, k.startsWith("enabled"))

// exists - Check any entry
{"a": 1, "b": 2}.exists(k, k == "a")    // → true
settings.exists(k, v, v == "production")

// filter - Filter entries
{"a": 1, "b": 2, "c": 3}.filter(k, v, v > 1)  // → {"b": 2, "c": 3}
users.filter(id, user, user.active)
```

---

## Type Conversion

### To Integer

```cel
int(3.9)                                // → 3 (truncates)
int("123")                              // → 123
int(100u)                               // → 100
int(timestamp("2023-01-01T00:00:00Z"))  // → epoch seconds
int(duration("1h"))                     // → 3600 (seconds)
```

### To Unsigned Integer

```cel
uint(42)                                // → 42u
uint("100")                             // → 100u
uint(3.14)                              // → 3u (truncates)
```

### To Double

```cel
double(42)                              // → 42.0
double("3.14")                          // → 3.14
```

### To String

```cel
string(123)                             // → "123"
string(3.14)                            // → "3.14"
string(true)                            // → "true"
string(b'hello')                        // → "hello"
string(timestamp("2023-01-01T00:00:00Z"))  // → "2023-01-01T00:00:00Z"
```

### To Bytes

```cel
bytes("hello")                          // → b"hello"
```

### To Boolean

```cel
bool("true")                            // → true
bool(1)                                 // → true
bool(0)                                 // → false
```

### To Dynamic Type

```cel
dyn(123)                                // → 123 as dynamic type
dyn("hello")                            // → "hello" as dynamic type
```

---

## Timestamp & Duration

### Creating Timestamps

```cel
// From string (RFC3339 format)
timestamp("2023-12-25T12:00:00Z")
timestamp("2023-12-25T12:00:00-05:00")

// From epoch seconds
timestamp(1678886400)
```

### Timestamp Methods

```cel
// Extract date components
timestamp("2023-12-25T12:30:45Z").getFullYear()       // → 2023
timestamp("2023-12-25T12:30:45Z").getMonth()          // → 11 (0-based: 0=Jan)
timestamp("2023-12-25T12:30:45Z").getDate()           // → 25 (day of month)
timestamp("2023-12-25T12:30:45Z").getDayOfWeek()      // → 0-6 (0=Sunday)
timestamp("2023-12-25T12:30:45Z").getDayOfYear()      // → 1-366

// Extract time components
timestamp("2023-12-25T12:30:45Z").getHours()          // → 12
timestamp("2023-12-25T12:30:45Z").getMinutes()        // → 30
timestamp("2023-12-25T12:30:45Z").getSeconds()        // → 45
timestamp("2023-12-25T12:30:45Z").getMilliseconds()   // → milliseconds

// With timezone
timestamp("2023-12-25T00:00:00Z").getDate("America/Los_Angeles")    // → 24
timestamp("2023-12-25T12:00:00Z").getHours("America/New_York")      // → 7
```

### Duration Operations

```cel
// Create duration
duration("1h")                          // 1 hour
duration("30m")                         // 30 minutes
duration("1h30m45s")                    // 1 hour, 30 minutes, 45 seconds
duration("500ms")                       // 500 milliseconds

// Duration methods
duration("3h").getHours()               // → 3
duration("90m").getMinutes()            // → 90
duration("1h30m").getSeconds()          // → 5400
duration("1500ms").getMilliseconds()    // → 1500

// Timestamp arithmetic
timestamp("2023-01-01T00:00:00Z") + duration("24h")     // Add duration
timestamp("2023-01-02T00:00:00Z") - timestamp("2023-01-01T00:00:00Z")  // → duration("24h")
```

---

## Macros

### has() - Field Existence

```cel
// Check if field exists (useful for protocol buffers)
has(message.field)
has(user.email)

// Check optional fields
has(request.auth) && request.auth.uid != ""
has(resource.owner) ? resource.owner : "anonymous"
```

### all() - Universal Quantifier

```cel
// For lists: check if all elements match
[2, 4, 6, 8].all(x, x % 2 == 0)         // → true
scores.all(s, s >= 0 && s <= 100)

// For maps: check if all key-value pairs match
{"a": 1, "b": 2}.all(k, v, v > 0)       // → true
```

### exists() - Existential Quantifier

```cel
// For lists: check if any element matches
[1, 2, 3, 4].exists(x, x > 3)           // → true
users.exists(u, u.role == "admin")

// For maps: check if any key-value pair matches
{"a": 1, "b": 2}.exists(k, v, v == 2)   // → true
```

### exists_one() - Unique Existence

```cel
// Check if exactly one element matches
[1, 2, 3, 4].exists_one(x, x == 3)      // → true
[1, 2, 2, 3].exists_one(x, x == 2)      // → false (two matches)
items.exists_one(i, i.primary == true)
```

### filter() - Selection

```cel
// Filter list elements
[1, 2, 3, 4, 5].filter(x, x > 2)        // → [3, 4, 5]
products.filter(p, p.price < 100.0)

// Filter map entries
{"a": 1, "b": 2, "c": 3}.filter(k, v, v > 1)  // → {"b": 2, "c": 3}
```

### map() - Transformation

```cel
// Transform list elements
[1, 2, 3].map(x, x * 2)                 // → [2, 4, 6]
users.map(u, u.name)                    // Extract field from each element
["hello", "world"].map(s, s.upperAscii())     // → ["HELLO", "WORLD"]
```

---

## Advanced Features

### Type Checking

```cel
// Get type
type(123)                               // → int
type("hello")                           // → string
type([1, 2, 3])                         // → list

// Type comparison
type(x) == int
type(data) == map
```

### Optional Values

```cel
// Create optional
optional.of(42)                         // Optional with value
optional.none()                         // Empty optional

// Check and extract
opt.hasValue()                          // → true/false
opt.value()                             // Get value or error if empty
opt.orValue(0)                          // Get value or default

// Optional field access
dyn({'a': 1}).?a.value()                // → 1
dyn({'a': 1}).?b.hasValue()             // → false
dyn({'a': 1}).?b.orValue(0)             // → 0
```

### Dynamic Typing

```cel
// Convert to dynamic type for heterogeneous collections
dyn('hello') == dyn('hello')            // → true

// Dynamic field access
dyn({'a': 1}).?a.value()                // → 1

// Use in format strings
'Hello, %s! You are %d.'.format(['World', dyn(30)])
```

---

## Practical Examples

### Validation

```cel
// Email validation
email.matches(r'^[\w.+-]+@[\w.-]+\.\w+$')

// Age validation
age >= 18 && age <= 120

// Required fields
has(user.name) && user.name.size() > 0 && has(user.email)

// Price range
product.price >= 0.0 && product.price <= 10000.0

// Password strength
password.size() >= 8 &&
password.matches(r'[A-Z]') &&
password.matches(r'[a-z]') &&
password.matches(r'[0-9]')
```

### Authorization Rules

```cel
// Check authenticated user
request.auth != null && request.auth.uid == resource.owner

// Role-based access
request.auth.token.role in ['admin', 'editor', 'viewer']

// Owner or admin
request.auth.uid == resource.owner || request.auth.token.role == 'admin'

// Time-based access
has(resource.validFrom) && has(resource.validUntil) &&
now() >= resource.validFrom && now() <= resource.validUntil
```

### Complex List Processing

```cel
// Calculate total of active items
items.filter(i, i.active).map(i, i.price).sum() < budget

// Check if all emails are valid
emails.all(e, e.matches(r'^[\w.+-]+@[\w.-]+\.\w+$'))

// Get unique active user IDs
items.filter(i, i.active).map(i, i.userId).distinct()

// Find if any high-priority task is incomplete
tasks.exists(t, t.priority == "high" && !t.completed)
```

### String Manipulation

```cel
// Clean and normalize path
request.path.trim().split('/').filter(s, s.size() > 0).join('_').lowerAscii()

// Extract domain from email
email.split('@')[1]

// Build full name
user.firstName + " " + user.lastName

// Format display text
"User: %s (ID: %d)".format([user.name, user.id])
```

### Date/Time Operations

```cel
// Check if timestamp is in the past
timestamp(dateString) < now()

// Check if within last 30 days
now() - timestamp(dateString) < duration("720h")  // 30 days

// Extract year from timestamp
timestamp(record.createdAt).getFullYear() >= 2020

// Business hours check (9 AM - 5 PM)
timestamp(event.time).getHours() >= 9 &&
timestamp(event.time).getHours() < 17
```

### Conditional Logic

```cel
// Nested conditionals
user.role == 'admin' ? 'full' :
  user.role == 'editor' ? 'write' :
  user.role == 'viewer' ? 'read' : 'none'

// Complex authorization
(user.role == 'admin') ||
(user.role == 'editor' && resource.owner == user.id) ||
(user.role == 'viewer' && resource.public == true)

// Default values
has(config.timeout) ? config.timeout : duration("30s")
```

### Working with Maps

```cel
// Check if all required keys exist
["name", "email", "age"].all(k, k in user)

// Get value with default
has(config.maxRetries) ? config.maxRetries : 3

// Count matching entries
config.filter(k, v, v == true).size()

// Build new map from list
users.map(u, {u.id: u.name})
```

---

## Tips & Best Practices

1. **Use raw strings** (`r"..."`) for regex patterns to avoid escaping backslashes
2. **Short-circuit evaluation**: `&&` and `||` operators short-circuit, use for performance
3. **Type safety**: Prefer explicit type conversion over relying on implicit conversion
4. **Null safety**: Always check with `has()` before accessing optional fields
5. **Macros are powerful**: Use `filter()`, `map()`, `all()`, and `exists()` for clean list/map operations
6. **Immutability**: All operations return new values; original values are never modified
7. **Performance**: Complex expressions are still evaluated in bounded time
8. **Readability**: Break complex expressions into smaller parts with intermediate variables if your CEL environment supports it

---

## Common Gotchas

- **Months are 0-based**: `getMonth()` returns 0 for January, 11 for December
- **Integer division**: `/` performs integer division when both operands are integers
- **String indexing**: Not all CEL implementations support direct string indexing; use `charAt()` instead
- **Empty checks**: Use `size() == 0` rather than comparing to `""` or `[]`
- **Timezone awareness**: Timestamp methods default to UTC unless timezone is specified

---

This guide covers all standard CEL built-in features. For environment-specific extensions or custom functions, consult your platform's documentation.
