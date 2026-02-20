# Architecture Diagram

## Three-Segment FIPS Architecture

The following diagrams illustrate the cloudflared-fips architecture across three network segments, with cryptographic module annotations at each boundary.

### End-to-End Data Flow

```mermaid
flowchart LR
    subgraph Segment1["Segment 1: Client"]
        A[End User Device]
        B[WARP Client]
        C["FIPS Crypto\n(OS Module)"]
    end

    subgraph Segment2["Segment 2: Cloudflare Edge"]
        D[Cloudflare Access]
        E[Gateway DNS/HTTP]
        F["Edge TLS\n(Cloudflare FIPS)"]
        G[DDoS Protection]
    end

    subgraph Segment3["Segment 3: Tunnel"]
        H["cloudflared\n(BoringCrypto #4407)"]
        I[Self-Test Engine]
        J["TLS 1.2+\nFIPS Cipher Suites"]
    end

    subgraph Origin["Origin Service"]
        K[Application Server]
        L[localhost:8443]
    end

    A --> B
    B -->|"TLS 1.2+ (FIPS)"| D
    D --> E
    E --> F
    F -->|"Encrypted Tunnel"| H
    H --> I
    H -->|"TLS (FIPS Ciphers)"| J
    J --> L
    L --> K

    style Segment1 fill:#e0f2fe,stroke:#0284c7
    style Segment2 fill:#fef3c7,stroke:#d97706
    style Segment3 fill:#dcfce7,stroke:#16a34a
    style Origin fill:#f3e8ff,stroke:#9333ea
```

### Cryptographic Module Deployment

```mermaid
flowchart TB
    subgraph BuildPipeline["Build Pipeline"]
        BP1["RHEL UBI 9\n(FIPS OpenSSL)"]
        BP2["Go 1.24\nGOEXPERIMENT=boringcrypto"]
        BP3["CGO_ENABLED=1"]
        BP4["Symbol Verification"]
        BP5["KAT Self-Tests"]
        BP6["Build Manifest"]
    end

    subgraph Runtime["Runtime Verification"]
        RT1["Startup Self-Test"]
        RT2["BoringCrypto Linked?"]
        RT3["OS FIPS Mode?"]
        RT4["Cipher Suite Check"]
        RT5["KAT Execution"]
    end

    subgraph CryptoModules["FIPS Crypto Modules"]
        CM1["BoringCrypto\nFIPS 140-2 #4407\nLevel 1"]
        CM2["AES-128/256-GCM"]
        CM3["SHA-256 / SHA-384"]
        CM4["HMAC-SHA-256"]
        CM5["ECDSA P-256/P-384"]
        CM6["RSA-2048+"]
        CM7["ECDH P-256/P-384"]
    end

    BP1 --> BP2 --> BP3 --> BP4 --> BP5 --> BP6
    RT1 --> RT2 --> RT3 --> RT4 --> RT5
    CM1 --> CM2
    CM1 --> CM3
    CM1 --> CM4
    CM1 --> CM5
    CM1 --> CM6
    CM1 --> CM7

    style BuildPipeline fill:#fef9c3,stroke:#ca8a04
    style Runtime fill:#dbeafe,stroke:#2563eb
    style CryptoModules fill:#dcfce7,stroke:#16a34a
```

### Network Security Boundaries

```mermaid
flowchart LR
    subgraph Internet["Public Internet"]
        direction TB
        U1[Remote User]
        U2[Branch Office]
    end

    subgraph CFEdge["Cloudflare Edge Network"]
        direction TB
        CF1["Access\n(Identity)"]
        CF2["Gateway\n(Filtering)"]
        CF3["WAF/DDoS\n(Protection)"]
        CF4["Edge TLS\nTermination"]
    end

    subgraph Tunnel["Encrypted Tunnel"]
        direction TB
        T1["cloudflared\n─────────────\nBoringCrypto #4407\nTLS 1.2+ Only\nFIPS Ciphers Only"]
    end

    subgraph PrivateNet["Private Network"]
        direction TB
        S1["Service A\n(localhost:8443)"]
        S2["Service B\n(localhost:9090)"]
        S3["Service C\n(localhost:3000)"]
    end

    U1 -->|"WARP TLS"| CF1
    U2 -->|"HTTPS"| CF1
    CF1 --> CF2 --> CF3 --> CF4
    CF4 -->|"Tunnel\n(QUIC/H2)"| T1
    T1 -->|"Local TLS"| S1
    T1 -->|"Local HTTP"| S2
    T1 -->|"Local HTTP"| S3

    style Internet fill:#fee2e2,stroke:#dc2626
    style CFEdge fill:#fef3c7,stroke:#d97706
    style Tunnel fill:#dcfce7,stroke:#16a34a
    style PrivateNet fill:#e0e7ff,stroke:#4f46e5
```

### Compliance Dashboard Architecture

```mermaid
flowchart TB
    subgraph Dashboard["Compliance Dashboard"]
        D1[React Frontend]
        D2[Vite Dev Server]
    end

    subgraph API["Dashboard API"]
        A1["GET /api/v1/compliance"]
        A2["GET /api/v1/manifest"]
        A3["GET /api/v1/selftest"]
    end

    subgraph DataSources["Data Sources"]
        DS1["Self-Test Engine"]
        DS2["Build Manifest"]
        DS3["Compliance Checker"]
    end

    D1 --> D2
    D2 --> A1
    D2 --> A2
    D2 --> A3
    A1 --> DS3
    A2 --> DS2
    A3 --> DS1

    style Dashboard fill:#dbeafe,stroke:#2563eb
    style API fill:#fef9c3,stroke:#ca8a04
    style DataSources fill:#dcfce7,stroke:#16a34a
```
