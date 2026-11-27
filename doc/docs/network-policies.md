# Network policies

Kubernetes `NetworkPolicy` objects restrict the traffic of the pods they select.
Katenary can generate one policy per service from the
[`katenary.v4/network-policy`](labels.md#katenaryv4network-policy) label.

## How a NetworkPolicy works

A policy is **namespaced** and selects pods with `podSelector` (in its own
namespace). Once a pod is selected for a direction, that direction is
**denied by default** and only the rules of the policy (and of the other
policies selecting the same pod) allow traffic. There is no "deny" rule: the
rules are additive allow-lists.

- **Ingress** is the traffic **entering** the selected pods.
- **Egress** is the traffic **leaving** the selected pods.
- Both directions are independent: a policy can isolate ingress, egress, or
  both.
- In an ingress/egress rule, all the fields of a peer must match
  (`podSelector` **and** `namespaceSelector` **and** `ipBlock` if several are
  set), while several peers or several rules are ORed.
- A `podSelector` without `namespaceSelector` only matches pods of the policy
  namespace. A `namespaceSelector` matches namespaces by their labels
  (`kubernetes.io/metadata.name` is set automatically by Kubernetes).
- Services are not selected directly: the policy applies to the **pods**
  behind the Service.

Typical consequences:

- If egress is isolated, **DNS must be allowed** explicitly, otherwise name
  resolution stops working.
- Pods of the same pod share their network namespace: `localhost` traffic is
  not affected by NetworkPolicies.
- Traffic to/from `hostNetwork` pods (some ingress controllers, the API server
  on managed clusters) may not match `podSelector`/`namespaceSelector`; use an
  `ipBlock` in that case.

## What Katenary generates

The label is **opt-in** and can be a boolean or an object:

```yaml
labels:
  katenary.v4/network-policy: "true"
```

With the default configuration, a policy selects the service pod and:

| Direction | Allowed by default |
| --- | --- |
| Ingress | other pods of the chart, on the service ports |
| Ingress | the ingress controller, when the service has a `katenary.v4/ingress` label, on the service ports |
| Egress | cluster DNS (`kube-system`, UDP/TCP 53) |
| Egress | other pods of the chart |
| Egress | the services declared in `depends_on`, on their ports |
| Egress | TCP 443 and the backend API server port 6443 for the Kubernetes API used by `depends_on` init containers |
| Egress | internet **only** with `allowInternet: true` |

The ingress controller namespaces are configured with `ingressNamespaces`
(`[traefik]`, `[ingress-nginx]`, ...). When the list is empty, any pod of the
chart namespace is allowed, which is correct when the controller runs in the
same namespace.

The generated policy can be disabled from `values.yaml`:

```yaml
web:
  networkPolicy:
    enabled: false
```

## A coherent example

`web` exposes HTTP through Traefik (in the `traefik` namespace), calls `api`
declared in `depends_on`, and needs to reach an external service in
`203.0.113.0/24`. `api` is only reachable from the chart pods.

```yaml
services:
  web:
    image: nginx:1.29
    ports:
      - "80:80"
    depends_on:
      - api
    labels:
      katenary.v4/ingress: |-
        hostname: app.example.com
        port: 80
        enabled: true
      katenary.v4/network-policy: |-
        enabled: true
        ingress:
          ingressNamespaces: [traefik]
        egress:
          extraPeers:
            - ipBlock:
                cidr: 203.0.113.0/24

  api:
    image: example.com/my/api:1.0
    ports:
      - "8080:8080"
    labels:
      katenary.v4/network-policy: "true"
```

### What is allowed and what is blocked

Both services are selected by a policy, so **everything is blocked in both
directions unless a rule below allows it**. "Chart pods" means any pod of the
chart (including `web` and `api`), not just the same service.

**`web` policy**

| Direction | Allowed | Blocked (examples) |
| --- | --- | --- |
| Ingress | chart pods to `web:80`; pods of the `traefik` namespace to `web:80` | Internet directly to `web:80`; other namespaces; any other port |
| Egress | DNS (`kube-dns:53`); chart pods (all ports); `api:8080` (`depends_on`); Kubernetes API `:443`/`:6443`; `203.0.113.0/24:443` | Internet on `:80`/`:443`; `api` on another port; any other external network |

**`api` policy**

| Direction | Allowed | Blocked (examples) |
| --- | --- | --- |
| Ingress | chart pods to `api:8080` | Traefik to `api:8080`; Internet; other namespaces; any other port |
| Egress | DNS (`kube-dns:53`); chart pods (all ports); Kubernetes API `:443`/`:6443` | Internet; `203.0.113.0/24`; any other external network |

Concrete checks:

| Traffic | Result |
| --- | --- |
| `web` → `api:8080` | ✅ allowed (chart pods + `depends_on`) |
| Traefik → `web:80` | ✅ allowed (ingress controller namespace) |
| Internet → `web:80` | ❌ blocked (must go through Traefik) |
| Traefik → `api:8080` | ❌ blocked (no ingress rule on `api`) |
| `web` → `203.0.113.0/24:443` | ✅ allowed (`extraPeers`) |
| `api` → `203.0.113.0/24:443` | ❌ blocked (only `web` has that peer) |
| `web` → `database:5432` | ❌ blocked, unless `database` is in `depends_on` or listed in `extraPeers` |
| DNS resolution from `web` or `api` | ✅ allowed (kube-dns:53) |
| `web` → Kubernetes API `:443`/`:6443` | ✅ allowed (`depends_on` init container) |

Containers of a `same-pod` service share their network namespace, so their
localhost traffic is not filtered by the policy.

## Options

```yaml
labels:
  katenary.v4/network-policy: |-
    enabled: true
    ingress:
      fromSameApp: true          # other chart pods
      fromIngress: true          # ingress controller, if the service has an ingress
      ingressNamespaces: [traefik]
      extraPeers:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: monitoring
    egress:
      allowDNS: true
      dnsNamespace: kube-system
      allowSameApp: true
      allowDependencies: true
      allowKubernetesAPI: true   # TCP 443 and 6443; disable if explicit API peers are used
      allowInternet: false
      extraPeers:
        - ipBlock:
            cidr: 198.51.100.0/24
            except: [198.51.100.42/32]
```

A `depends_on` init container calls the Kubernetes API by default. The backend
API server port (6443 with kubeadm/kind) is allowed in addition to the service
port 443, because some CNIs evaluate the egress policy after the service DNAT.
If you disable `allowKubernetesAPI`, add an `ipBlock` (or the appropriate peer)
for your API server, otherwise the dependency wait will fail.

### Same-pod services

A `same-pod` service is merged into the target deployment. Only the target
service gets a NetworkPolicy; the merged containers are selected by the target
pod, and their localhost traffic is not restricted by the policy.
