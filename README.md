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
* TCP connections: incoming and outgoing connection events (connect/accept)
* Syscalls: system calls the application uses
* Files touched: list of files the containers are touching (and their access mode) - *yet to be implemented*

### Example of an application profile

```yaml
apiVersion: kubescape.io/v1
kind: ApplicationProfile
metadata:
  name: pod-frontend-bfdf66596-rd4qn
  namespace: hipster
spec:
  containers:
  - execs:
    - path: /src/server
    name: server
    networkActivity:
      incoming:
        tcpconnections:
        - rawconnection:
            destip: ::ffff:10.244.0.23
            destport: 8080
            sourceip: ::ffff:10.244.0.1
            sourceport: 0
        - rawconnection:
            destip: ::ffff:10.244.0.23
            destport: 8080
            sourceip: ::ffff:10.244.0.22
            sourceport: 0
      outgoing:
        tcpconnections:
        - rawconnection:
            destip: 10.100.181.6
            destport: 7000
            sourceip: 10.244.0.23
            sourceport: 0
        - rawconnection:
            destip: 10.109.45.84
            destport: 7070
            sourceip: 10.244.0.23
            sourceport: 0
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


