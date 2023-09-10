# Kubescape Application profiler

## Intro

This is a **lab project** made by the Kubescape team whose goal is to generate an application profile containing metrics/properties for Kubernetes workloads based on runtime behavior.

There are multiple metrics and properties of an application that are not available in Kubernetes API and or other standard data sources, for example, the executables that run in a container, or the TCP connection that it does. To access these kinds of information the operator needs to have observability tooling in place.

This Application profiler uses [Inspektor Gadget](https://www.inspektor-gadget.io/) to collect information about running workloads in Kubernetes and compile them into a [Kubernetes custom resource](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).

### Use cases

This information can help Kubescape (or other tooling) to:
* Verify the workload configuration (for example: determine if a workload needs to be privileged)
* Build hardening based on workload behavior (security context, network policies, seccomp and etc)

Based on Kubernetes API, it enables to GitOps and easy transition of this information in the ecosystem.

### Data collected

* Execve events: the process starts with arguments
* File access: list of files that were opened in the container (and their access mode)
* Network connections: incoming and outgoing connection events
* DNS: DNS requests and responses by the container - *Right now limited because of [this](https://github.com/inspektor-gadget/inspektor-gadget/issues/2008) issue*
* Syscalls: system calls the application uses
* Linux capabilities requested by the containerized processes


### Example of an application profile

```yaml
apiVersion: kubescape.io/v1
kind: ApplicationProfile
metadata:
  creationTimestamp: "2023-09-10T06:42:24Z"
  generation: 2
  name: deployment-frontend
  namespace: hipster
  resourceVersion: "142668"
  uid: 8419da2a-0584-4be6-9a37-0efd0f2c7b97
spec:
  containers:
  - capabilities:
    - caps:
      - NET_ADMIN
      syscall: read
    - caps:
      - NET_ADMIN
      syscall: openat
    dns:
    - dnsName: metadata.google.internal.
    - dnsName: adservice.hipster.svc.cluster.local.
    - dnsName: cartservice.hipster.svc.cluster.local.
    - dnsName: checkoutservice.hipster.svc.cluster.local.
    - dnsName: currencyservice.hipster.svc.cluster.local.
    - dnsName: shippingservice.hipster.svc.cluster.local.
    - dnsName: productcatalogservice.hipster.svc.cluster.local.
    - dnsName: recommendationservice.hipster.svc.cluster.local.
    execs:
    - path: /src/server
    name: server
    networkActivity:
      incoming:
      - dstEndpoint: 10.244.0.1
        port: 8080
        protocol: tcp
      - dstEndpoint: 10.244.0.109
        port: 8080
        protocol: tcp
      outgoing:
      - dstEndpoint: 169.254.169.254
        port: 80
        protocol: tcp
      - dstEndpoint: 10.97.13.57
        port: 3550
        protocol: tcp
      - dstEndpoint: 10.96.112.73
        port: 5050
        protocol: tcp
      - dstEndpoint: 10.97.138.113
        port: 7000
        protocol: tcp
      - dstEndpoint: 10.102.37.192
        port: 7070
        protocol: tcp
      - dstEndpoint: 10.108.166.241
        port: 8080
        protocol: tcp
      - dstEndpoint: 10.108.135.173
        port: 9555
        protocol: tcp
      - dstEndpoint: 10.103.31.34
        port: 50051
        protocol: tcp
      - dstEndpoint: 10.96.0.10
        port: 53
        protocol: udp
    opens:
    - flags:
      - O_RDONLY
      - O_CLOEXEC
      path: /etc/hosts
    - flags:
      - O_RDONLY
      - O_CLOEXEC
      path: /src/server
    - flags:
      - O_RDONLY
      - O_CLOEXEC
      path: /src/templates
    - flags:
      - O_RDONLY
      - O_CLOEXEC
      path: /etc/resolv.conf
    - flags:
      - O_RDONLY
      - O_CLOEXEC
      path: /etc/nsswitch.conf
    - flags:
      - O_RDONLY
      - O_CLOEXEC
      path: /src/templates/ad.html
    - flags:
      - O_RDONLY
      - O_CLOEXEC
      path: /src/templates/cart.html
    - flags:
      - O_RDONLY
      - O_CLOEXEC
      path: /src/templates/home.html
    - flags:
      - O_RDONLY
      - O_CLOEXEC
      path: /src/templates/error.html
    - flags:
      - O_RDONLY
      - O_CLOEXEC
      path: /src/templates/order.html
    - flags:
      - O_RDONLY
      - O_CLOEXEC
      path: /src/templates/footer.html
    - flags:
      - O_RDONLY
      - O_CLOEXEC
      path: /src/templates/header.html
    - flags:
      - O_RDONLY
      - O_CLOEXEC
      path: /src/templates/product.html
    - flags:
      - O_RDONLY
      - O_CLOEXEC
      path: /proc/sys/net/core/somaxconn
    - flags:
      - O_RDONLY
      - O_CLOEXEC
      path: /src/templates/recommendations.html
    - flags:
      - O_RDONLY
      path: /sys/kernel/mm/transparent_hugepage/hpage_pmd_size
    syscalls:
    - accept4
    - arch_prctl
    - bind
    - brk
    - capget
    - capset
    - chdir
    - clone
    - close
    - connect
    - epoll_create1
    - epoll_ctl
    - epoll_pwait
    - execve
    - faccessat2
    - fchown
    - fcntl
    - fstat
    - fstatfs
    - futex
    - getdents64
    - getpeername
    - getpid
    - getppid
    - getrandom
    - getrlimit
    - getsockname
    - getsockopt
    - gettid
    - listen
    - madvise
    - membarrier
    - mmap
    - mprotect
    - nanosleep
    - newfstatat
    - openat
    - pipe2
    - prctl
    - pread64
    - read
    - readlinkat
    - rt_sigaction
    - rt_sigprocmask
    - rt_sigreturn
    - sched_getaffinity
    - sched_yield
    - set_tid_address
    - setgid
    - setgroups
    - setsockopt
    - setuid
    - sigaltstack
    - socket
    - tgkill
    - uname
    - write
```

## Install

Simple installation:
```bash
kubectl apply -f https://raw.githubusercontent.com/kubescape/kapprofiler/main/etc/app-profile.crd.yaml
kubectl apply -f https://raw.githubusercontent.com/kubescape/kapprofiler/main/deployment/deployment.yaml
```

Voila 😉

## Usage

The profile starts recording events every time a container starts and updates profiles evert 2 minutes.

To see your application profiles run
```bash
kubectl get applicationprofiles.kubescape.io -A
```


