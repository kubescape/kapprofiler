#pragma once

#include "../../../../include/types.h"

#ifndef TASK_COMM_LEN
#define TASK_COMM_LEN 16
#endif
#define INVALID_UID ((uid_t)-1)

struct event {
    gadget_timestamp timestamp;
    gadget_mntns_id mntns_id;
    __u32 pid;
    __u32 ppid;
    __u32 uid;
    __u32 gid;
    __u8 comm[TASK_COMM_LEN];
};
