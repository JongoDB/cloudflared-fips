# Architecture Diagrams

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
        F["Edge TLS\n(BoringSSL — FedRAMP\ninherited, not\nindependently validated)"]
        G[DDoS Protection]
    end

    subgraph Segment3["Segment 3: Tunnel"]
        H["cloudflared-fips\n(BoringCrypto #4407/#4735)"]
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
    F -->|"Encrypted Tunnel\n(QUIC/H2)"| H
    H --> I
    H -->|"TLS (FIPS Ciphers)"| J
    J --> L
    L --> K

    style Segment1 fill:#e0f2fe,stroke:#0284c7
    style Segment2 fill:#fef3c7,stroke:#d97706
    style Segment3 fill:#dcfce7,stroke:#16a34a
    style Origin fill:#f3e8ff,stroke:#9333ea
```

### Verification Method per Segment

Each segment has a different level of verifiability:

```mermaid
flowchart TB
    subgraph Client["Segment 1: Client"]
        C1["OS FIPS Mode"]
        C2["Browser Cipher Suites"]
        C3["Device Posture"]
    end

    subgraph Edge["Segment 2a: Cloudflare Edge"]
        E1["Cipher Config"]
        E2["Min TLS Version"]
        E3["Access Policy"]
        E4["Edge Crypto Module"]
    end

    subgraph Tunnel["Segment 2b: Tunnel"]
        T1["BoringCrypto Active"]
        T2["Self-Test KATs"]
        T3["OS FIPS Mode"]
        T4["Binary Integrity"]
    end

    subgraph Local["Segment 3: Local"]
        L1["TLS Handshake"]
        L2["Cipher Negotiation"]
        L3["Cert Validity"]
    end

    C1 -.->|Reported| C1
    C2 -.->|Probe| C2
    C3 -.->|Reported| C3

    E1 -.->|API| E1
    E2 -.->|API| E2
    E3 -.->|API| E3
    E4 -.->|"Inherited\n(FedRAMP)"| E4

    T1 -.->|Direct| T1
    T2 -.->|Direct| T2
    T3 -.->|Direct| T3
    T4 -.->|Direct| T4

    L1 -.->|Probe| L1
    L2 -.->|Probe| L2
    L3 -.->|Probe| L3

    style Client fill:#e0f2fe,stroke:#0284c7
    style Edge fill:#fef3c7,stroke:#d97706
    style Tunnel fill:#dcfce7,stroke:#16a34a
    style Local fill:#f3e8ff,stroke:#9333ea
```

---

## Deployment Tiers

### Tier 1: Standard Cloudflare Tunnel

```mermaid
flowchart LR
    subgraph Internet
        U[Client]
    end

    subgraph CF["Cloudflare Edge"]
        CF1["TLS Termination\n(BoringSSL — inherited)"]
        CF2["Access + WAF"]
    end

    subgraph Customer["Customer Infrastructure"]
        T["cloudflared-fips\n(BoringCrypto — validated)"]
        O["Origin Service"]
    end

    U -->|"TLS 1.2+"| CF1
    CF1 --> CF2
    CF2 -->|"QUIC/H2 Tunnel"| T
    T -->|"Local TLS"| O

    style CF fill:#fef3c7,stroke:#d97706
    style Customer fill:#dcfce7,stroke:#16a34a
```

**Edge crypto:** Inherited from Cloudflare FedRAMP Moderate authorization. Not independently verifiable.
**Key management:** Cloudflare-managed edge keys.
**Gap:** Edge crypto module FIPS validation cannot be independently confirmed.

### Tier 2: Cloudflare's Official FIPS 140 Level 3 Architecture

This is [Cloudflare's reference architecture for FIPS 140 Level 3 compliance](https://developers.cloudflare.com/reference-architecture/diagrams/security/fips-140-3/). The tunnel carries **both application traffic AND cryptographic key operations** for Keyless SSL.

```mermaid
flowchart LR
    subgraph Internet
        U[Client]
    end

    subgraph CF["Cloudflare Edge\n(Regional Services — US FedRAMP DCs only)"]
        CF1["TLS Termination\n(needs key operation)"]
        CF2["Access + WAF"]
    end

    subgraph Customer["Customer Infrastructure"]
        T["cloudflared-fips\n(BoringCrypto — validated)\n━━━━━━━━━━━━━━━━\nCarries app traffic +\nKeyless SSL key ops"]
        KL["Keyless SSL Module\n(software proxy)"]
        HSM["HSM\n(FIPS 140-2 L3)\nPKCS#11"]
        O["Origin Service"]
    end

    U -->|"TLS 1.2+"| CF1
    CF1 -->|"Key operation\nrequest"| CF2
    CF2 -->|"QUIC/H2 Tunnel"| T
    T -->|"Key ops"| KL
    KL -->|"PKCS#11"| HSM
    HSM -->|"Signed result"| KL
    KL -->|"Key response"| T
    T -->|"App traffic"| O

    style CF fill:#fef3c7,stroke:#d97706
    style Customer fill:#dcfce7,stroke:#16a34a
    style HSM fill:#fce7f3,stroke:#db2777
```

**Why the tunnel is critical in Tier 2:** Every TLS handshake at the Cloudflare edge triggers a key operation that flows through this tunnel to the customer's HSM. The private key **never leaves the HSM** — only the signed result returns. A compromised or non-validated tunnel binary could intercept these key operations.

**Edge crypto:** Bulk encryption (AES-GCM) still performed by Cloudflare's edge BoringSSL. Key material protected by customer's HSM.
**Key management:** Customer's FIPS 140-2 Level 3 HSM via PKCS#11.

**Supported HSMs:**
- AWS CloudHSM
- Azure Dedicated HSM / Azure Managed HSM
- Entrust nShield Connect
- Fortanix Data Security Manager
- Google Cloud HSM
- IBM Cloud HSM
- SoftHSMv2 (development/testing only)

### Tier 3: Self-Hosted FIPS Edge Proxy

```mermaid
flowchart LR
    subgraph Internet
        U[Client]
    end

    subgraph GovCloud["GovCloud (AWS/Azure/Google)"]
        FP["FIPS Proxy\n(BoringCrypto — validated)\nTLS termination +\nClientHello inspection +\nJA4 fingerprinting"]
    end

    subgraph CF["Cloudflare (optional)"]
        CF1["WAF / DDoS"]
    end

    subgraph Customer["Customer Infrastructure"]
        T["cloudflared-fips\n(BoringCrypto — validated)"]
        O["Origin Service"]
    end

    U -->|"TLS 1.2+\n(FIPS ciphers)"| FP
    FP -->|"Optional"| CF1
    CF1 -->|"Tunnel"| T
    FP -->|"Direct"| T
    T -->|"Local TLS"| O

    style GovCloud fill:#dbeafe,stroke:#2563eb
    style CF fill:#fef3c7,stroke:#d97706
    style Customer fill:#dcfce7,stroke:#16a34a
```

**Edge crypto:** Fully controlled — customer's FIPS proxy terminates TLS with a validated module.
**Key management:** Customer-managed (local cert/key, or HSM).
**No gap:** Every TLS termination point uses a FIPS-validated module the customer controls.

---

## Cryptographic Module Matrix

```mermaid
flowchart TB
    subgraph Backends["FIPS Crypto Backends (pkg/fipsbackend/)"]
        BC["BoringCrypto\n━━━━━━━━━━━━\n140-2: #3678, #4407\n140-3: #4735\nLinux amd64/arm64\nCGO required"]
        GN["Go Native FIPS\n━━━━━━━━━━━━\n140-3: CAVP A6650\n(CMVP pending)\nAll platforms\nNo CGO"]
        SC["SystemCrypto\n━━━━━━━━━━━━\nWindows: CNG\nmacOS: CommonCrypto\nLinux: OpenSSL\nMS Go fork"]
    end

    subgraph Detection["Auto-Detection (Detect())"]
        D1["Check TLS cipher\nrestriction heuristic"]
        D2["Check GODEBUG\n=fips140=on"]
        D3["Check GOEXPERIMENT\n=systemcrypto"]
    end

    D1 --> BC
    D2 --> GN
    D3 --> SC

    subgraph PQC["Post-Quantum Readiness"]
        PQ1["BoringSSL: ML-KEM\n(Kyber) supported"]
        PQ2["Go 1.24+: ML-KEM\nvia crypto/mlkem"]
        PQ3["Cloudflare edge:\nPQC for origin conns"]
    end

    style Backends fill:#dcfce7,stroke:#16a34a
    style Detection fill:#dbeafe,stroke:#2563eb
    style PQC fill:#fef3c7,stroke:#d97706
```

---

## Cryptographic Module Deployment

```mermaid
flowchart TB
    subgraph BuildPipeline["Build Pipeline"]
        BP1["RHEL UBI 9\n(FIPS OpenSSL)"]
        BP2["Go 1.24\nGOEXPERIMENT=boringcrypto"]
        BP3["CGO_ENABLED=1"]
        BP4["Symbol Verification"]
        BP5["KAT Self-Tests"]
        BP6["Build Manifest"]
        BP7["SBOM Generation\n(CycloneDX + SPDX)"]
        BP8["Artifact Signing\n(GPG + cosign)"]
    end

    subgraph Runtime["Runtime Verification"]
        RT1["Startup Self-Test"]
        RT2["BoringCrypto Linked?"]
        RT3["OS FIPS Mode?"]
        RT4["Cipher Suite Check"]
        RT5["KAT Execution"]
        RT6["Binary Integrity\n(SHA-256 vs manifest)"]
    end

    subgraph CryptoModules["FIPS Crypto Modules"]
        CM1["BoringCrypto\nFIPS 140-2 #4407\nFIPS 140-3 #4735"]
        CM2["AES-128/256-GCM"]
        CM3["SHA-256 / SHA-384"]
        CM4["HMAC-SHA-256"]
        CM5["ECDSA P-256/P-384"]
        CM6["RSA-2048+"]
        CM7["ECDH P-256/P-384"]
        CM8["ML-KEM (Kyber)\n(post-quantum)"]
    end

    BP1 --> BP2 --> BP3 --> BP4 --> BP5 --> BP6 --> BP7 --> BP8
    RT1 --> RT2 --> RT3 --> RT4 --> RT5 --> RT6
    CM1 --> CM2
    CM1 --> CM3
    CM1 --> CM4
    CM1 --> CM5
    CM1 --> CM6
    CM1 --> CM7
    CM1 -.->|"future"| CM8

    style BuildPipeline fill:#fef9c3,stroke:#ca8a04
    style Runtime fill:#dbeafe,stroke:#2563eb
    style CryptoModules fill:#dcfce7,stroke:#16a34a
```

---

## Network Security Boundaries

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
        CF5["Keyless SSL\n(key ops via tunnel)"]
    end

    subgraph Tunnel["Encrypted Tunnel"]
        direction TB
        T1["cloudflared-fips\n─────────────\nBoringCrypto #4407/#4735\nTLS 1.2+ Only\nFIPS Ciphers Only\n─────────────\nCarries: app traffic\n+ Keyless SSL key ops"]
    end

    subgraph PrivateNet["Private Network"]
        direction TB
        S1["Service A\n(localhost:8443)"]
        S2["Keyless Module\n→ HSM (PKCS#11)"]
        S3["Service B\n(localhost:3000)"]
    end

    U1 -->|"WARP TLS"| CF1
    U2 -->|"HTTPS"| CF1
    CF1 --> CF2 --> CF3 --> CF4
    CF4 --> CF5
    CF5 -->|"Tunnel\n(QUIC/H2)"| T1
    T1 -->|"Local TLS"| S1
    T1 -->|"Key Ops"| S2
    T1 -->|"Local HTTP"| S3

    style Internet fill:#fee2e2,stroke:#dc2626
    style CFEdge fill:#fef3c7,stroke:#d97706
    style Tunnel fill:#dcfce7,stroke:#16a34a
    style PrivateNet fill:#e0e7ff,stroke:#4f46e5
```

---

## Compliance Dashboard Architecture

```mermaid
flowchart TB
    subgraph Dashboard["Compliance Dashboard"]
        D1[React Frontend]
        D2["Sunset Banner\n+ Tier Badge"]
        D3["39 Checklist Items\n+ Verification Badges"]
    end

    subgraph API["Dashboard API (Go)"]
        A1["GET /api/v1/compliance"]
        A2["GET /api/v1/manifest"]
        A3["GET /api/v1/selftest"]
        A4["GET /api/v1/events (SSE)"]
        A5["GET /api/v1/migration"]
        A6["GET /api/v1/deployment"]
        A7["GET /api/v1/clients"]
        A8["GET /api/v1/signatures"]
    end

    subgraph DataSources["Data Sources"]
        DS1["Self-Test Engine\n(KATs, cipher check)"]
        DS2["Build Manifest"]
        DS3["Live Compliance\nChecker (local)"]
        DS4["Cloudflare API\n(zone settings)"]
        DS5["TLS Inspector\n(ClientHello)"]
        DS6["FIPS Backend\nDetector"]
    end

    D1 --> D2
    D1 --> D3
    D1 --> A1 & A2 & A3 & A4
    A1 --> DS3
    A1 --> DS4
    A2 --> DS2
    A3 --> DS1
    A5 --> DS6
    A7 --> DS5

    style Dashboard fill:#dbeafe,stroke:#2563eb
    style API fill:#fef9c3,stroke:#ca8a04
    style DataSources fill:#dcfce7,stroke:#16a34a
```

---

## Keyless SSL Key Operation Flow (Tier 2 Detail)

This diagram shows the detailed flow of a single TLS handshake when Keyless SSL is configured with a Cloudflare Tunnel. This is [Cloudflare's reference architecture for FIPS 140 Level 3](https://developers.cloudflare.com/reference-architecture/diagrams/security/fips-140-3/).

```mermaid
sequenceDiagram
    participant Client
    participant Edge as Cloudflare Edge
    participant Tunnel as cloudflared-fips
    participant Keyless as Keyless Module
    participant HSM as HSM (FIPS 140-2 L3)

    Client->>Edge: ClientHello (SNI: keyless.example.com)
    Edge->>Edge: Detect keyless certificate
    Edge->>Edge: Need private key operation

    Edge->>Tunnel: Key operation request (via tunnel)
    Note over Tunnel: Tunnel encrypted with<br/>BoringCrypto (FIPS validated)

    Tunnel->>Keyless: Forward key operation
    Keyless->>HSM: PKCS#11 sign request
    Note over HSM: Private key NEVER<br/>leaves the HSM

    HSM->>Keyless: Signed result
    Keyless->>Tunnel: Key operation response
    Tunnel->>Edge: Return signed result (via tunnel)

    Edge->>Client: ServerHello + Certificate
    Edge->>Client: Application data (AES-GCM)
    Note over Edge,Client: Bulk encryption uses<br/>edge BoringSSL (FedRAMP inherited)
```
