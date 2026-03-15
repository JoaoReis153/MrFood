``` mermaid
flowchart TB
    A("User Client") <-- Rest requests --> B("API Gateway
    - RTT limiting
    - load balancing
    - SSL termination")
    C("Business Client (restaurants)") <-- Rest requests --> B
    D("Notification Service (PubSub / SSE)") <--> B
    D --> O[("PostgreSQL Database")]
    E("Search Service") <-- Use cases: 4, 5, 10 --> B
    E <--> N[("Elastic search")] & O
    F("restaurant details review service pagination w/cursor") <-- Use cases: 2,3 --> B
    F <--> O & G(["REDIS instance (keep trending restaurants in case of (?))"])
    H("Auth service") <-- (Login, register, logout) --> B
    H <--> O
    I("Review service") <-- GET, POST, PUT, DELETE reviews --> B
    I <-- Each review updates sync, average and number of ratings --> O
    J("Booking Service") <--> B & M(["REDIS (keep booking details TTL 10min)"]) & K("3rd party service for payments") & K
    J <-- ACID --> O
    K <--> L("Business service")
    O -- Change data capture (CDC) --> N
    O --> P(("Blob storage"))
    P <--> Q("Content delivery network CDN")
    P <-- "Pre-signed URL" --> C
    Q <-- Download media --> A
    I <--> J
```