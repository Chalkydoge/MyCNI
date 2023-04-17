// SPDX-License-Identifier: (LGPL-2.1 OR BSD-2-Clause)

/*
    user-space code for handling capture packets run through
    veth-pair end coming out of container(netns)
    
    Copyright (c) 20xx
*/

#include <signal.h>
#include <unistd.h>
#include "veth_ingress.skel.h"

#define LO_IFINDEX	    1

static volatile sig_atomic_t exiting = 0;

static void sig_int(int signo)
{
	exiting = 1;
}

static int libbpf_print_fn(enum libbpf_print_level level, const char *format, va_list args)
{
	return vfprintf(stderr, format, args);
}

int main(int argc, char **argv)
{
	// Assume Go program will call this program with ifindex given.
	if (argc != 2) {
		printf("Please input the network device's ifindex!\n");
		goto cleanup;
	}
	
	int IFINDEX = atoi(argv[1]);

	// declare libbpf options, attach tc hook to tc ingress,
	// device index is the LXC_1_IFINDEX(handling packets from lxc1)
    DECLARE_LIBBPF_OPTS(bpf_tc_hook, tc_hook,
		.ifindex = IFINDEX, .attach_point = BPF_TC_INGRESS);
	
	DECLARE_LIBBPF_OPTS(bpf_tc_opts, tc_opts,
		.handle = 1, .priority = 1);
	bool hook_created = false;
	struct veth_ingress_bpf *skel;
	int err;

    // set libbpf_print_function
	libbpf_set_print(libbpf_print_fn);

	skel = veth_ingress_bpf__open_and_load();
	if (!skel) {
		fprintf(stderr, "Failed to open BPF skeleton\n");
		return 1;
	}

	/* The hook (i.e. qdisc) may already exists because:
	 *   1. it is created by other processes or users
	 *   2. or since we are attaching to the TC ingress ONLY,
	 *      bpf_tc_hook_destroy does NOT really remove the qdisc,
	 *      there may be an egress filter on the qdisc
	 */
	err = bpf_tc_hook_create(&tc_hook);
	if (!err)
		hook_created = true;
    
	if (err && err != -EEXIST) {
		fprintf(stderr, "Failed to create TC hook: %d\n", err);
		goto cleanup;
	}

    // bpftool generated skel would contain this part of info
	tc_opts.prog_fd = bpf_program__fd(skel->progs.veth_ingress);
	err = bpf_tc_attach(&tc_hook, &tc_opts);
	if (err) {
		fprintf(stderr, "Failed to attach TC: %d\n", err);
		goto cleanup;
	}

	if (signal(SIGINT, sig_int) == SIG_ERR) {
		err = errno;
		fprintf(stderr, "Can't set signal handler: %s\n", strerror(errno));
		goto cleanup;
	}

	printf("Successfully started! Please run `sudo cat /sys/kernel/debug/tracing/trace_pipe` "
	       "to see output of the BPF program.\n");

	while (!exiting) {
		fprintf(stderr, ".");
		sleep(1);
	}

    // clearing everything
	tc_opts.flags = tc_opts.prog_fd = tc_opts.prog_id = 0;
	err = bpf_tc_detach(&tc_hook, &tc_opts);
	if (err) {
		fprintf(stderr, "Failed to detach TC: %d\n", err);
		goto cleanup;
	}

cleanup:
	if (hook_created)
		bpf_tc_hook_destroy(&tc_hook);
	veth_ingress_bpf__destroy(skel);
	return -err;
}
