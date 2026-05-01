package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const (
	cgroupRoot = "/sys/fs/cgroup"
	cgroupName = "go-container-scratch"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "setup-cgroup":
		setupCgroup()
	case "enter-cgroup":
		enterCgroup()
	case "check-rootless":
		checkRootless()
	case "run":
		run()
	case "child":
		child()
	case "help", "-h", "--help":
		usage()
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`usage:
  sudo go run main.go setup-cgroup [uid] [gid]
  sudo go run main.go enter-cgroup <command> [args...]
  go run main.go check-rootless
  ./go-container run <rootfs> <command> [args...]

example:
  sudo go run main.go setup-cgroup
  go build -o go-container
  sudo ./go-container enter-cgroup ./go-container run ./ubuntu-2404-rootfs /bin/bash
  go run main.go check-rootless`)
}

func run() {
	if len(os.Args) < 4 {
		usage()
		os.Exit(1)
	}

	rootfs := os.Args[2]
	command := os.Args[3]
	args := os.Args[4:]

	fmt.Printf("Running Go Container rootfs=%s command=%s args=%v\n", rootfs, command, args)

	if !isInDelegatedCgroup() {
		fmt.Fprintf(os.Stderr, "process is not running inside delegated cgroup %s\n", delegatedCgroupFsPath())
		fmt.Fprintln(os.Stderr, "start it with: sudo ./go-container enter-cgroup ./go-container run <rootfs> <command> [args...]")
		os.Exit(1)
	}

	cgroupPath, err := createContainerCgroup()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create delegated cgroup: %v\n", err)
		fmt.Fprintf(os.Stderr, "run first: sudo %s setup-cgroup\n", os.Args[0])
		os.Exit(1)
	}
	defer os.Remove(cgroupPath)

	childArgs := append([]string{"child", rootfs, command}, args...)
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to find current executable: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(exe, childArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:   syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWUSER,
		Unshareflags: syscall.CLONE_NEWNS,
		Credential:   &syscall.Credential{Uid: 0, Gid: 0},
		UidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getuid(), Size: 1},
		},
		GidMappingsEnableSetgroups: false,
		GidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getgid(), Size: 1},
		},
	}

	if err := cmd.Start(); err != nil {
		printStartError(err)
		os.Exit(1)
	}
	if err := writeFile(filepath.Join(cgroupPath, "cgroup.procs"), strconv.Itoa(cmd.Process.Pid)); err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		printCgroupMoveError(cgroupPath, err)
		os.Exit(1)
	}
	must(cmd.Wait())
}

func child() {
	if len(os.Args) < 4 {
		usage()
		os.Exit(1)
	}

	rootfs := os.Args[2]
	command := os.Args[3]
	args := os.Args[4:]

	fmt.Printf("Child Running rootfs=%s command=%s args=%v\n", rootfs, command, args)

	cmd := exec.Command(command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	must(syscall.Sethostname([]byte("mario21ic-container")))
	must(syscall.Mount("", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, ""))
	must(syscall.Chroot(rootfs))
	must(os.Chdir("/"))

	must(syscall.Mount("proc", "proc", "proc", 0, ""))
	must(syscall.Mount("mario", "mario21ic_temp", "tmpfs", 0, ""))

	err := cmd.Run()

	must(syscall.Unmount("proc", 0))
	must(syscall.Unmount("mario21ic_temp", 0))
	must(err)
}

func setupCgroup() {
	uid, gid := delegatedUser()
	if len(os.Args) >= 3 {
		uid = mustAtoi(os.Args[2])
	}
	if len(os.Args) >= 4 {
		gid = mustAtoi(os.Args[3])
	}

	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "setup-cgroup must run as root because it writes to /sys/fs/cgroup")
		os.Exit(1)
	}

	basePath := filepath.Join(cgroupRoot, cgroupName)
	userPath := filepath.Join(basePath, strconv.Itoa(uid))

	must(enableControllers(cgroupRoot, "pids", "memory"))
	must(os.MkdirAll(basePath, 0755))
	must(enableControllers(basePath, "pids", "memory"))
	must(os.MkdirAll(userPath, 0755))
	must(enableControllers(userPath, "pids", "memory"))
	must(chownCgroup(userPath, uid, gid))

	fmt.Printf("Delegated cgroup ready: %s uid=%d gid=%d\n", userPath, uid, gid)
}

func enterCgroup() {
	if len(os.Args) < 3 {
		usage()
		os.Exit(1)
	}

	uid, gid := delegatedUser()
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "enter-cgroup must run as root so it can move the first process into the delegated cgroup")
		os.Exit(1)
	}

	userPath := delegatedCgroupFsPathForUID(uid)
	if _, err := os.Stat(userPath); err != nil {
		fmt.Fprintf(os.Stderr, "delegated cgroup is not ready: %v\n", err)
		fmt.Fprintln(os.Stderr, "run first: sudo go run main.go setup-cgroup")
		os.Exit(1)
	}

	runnerPath := filepath.Join(userPath, "runner-"+strconv.Itoa(os.Getpid()))
	must(os.Mkdir(runnerPath, 0755))
	must(chownCgroup(runnerPath, uid, gid))
	must(writeFile(filepath.Join(runnerPath, "cgroup.procs"), strconv.Itoa(os.Getpid())))

	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)},
	}

	must(cmd.Start())
	must(cmd.Wait())
}

func checkRootless() {
	uid, _ := delegatedUser()

	printKernelValue("/proc/sys/kernel/unprivileged_userns_clone")
	printKernelValue("/proc/sys/user/max_user_namespaces")
	printKernelValue("/proc/sys/kernel/apparmor_restrict_unprivileged_userns")
	printStatusValue("Seccomp")
	printCurrentCgroup()

	if _, err := os.Stat(delegatedCgroupFsPathForUID(uid)); err != nil {
		fmt.Printf("delegated cgroup: missing (%s)\n", delegatedCgroupFsPathForUID(uid))
		fmt.Println("run first: sudo go run main.go setup-cgroup")
		return
	}
	fmt.Printf("delegated cgroup: ok (%s)\n", delegatedCgroupFsPathForUID(uid))
	if isInDelegatedCgroupForUID(uid) {
		fmt.Println("current process cgroup: inside delegated subtree")
		return
	}
	fmt.Println("current process cgroup: outside delegated subtree")
	fmt.Println("use: sudo ./go-container enter-cgroup ./go-container run ./ubuntu-2404-rootfs /bin/bash")
}

func createContainerCgroup() (string, error) {
	path := filepath.Join(delegatedCgroupFsPath(), "container-"+strconv.Itoa(os.Getpid()))
	if err := os.Mkdir(path, 0755); err != nil {
		return "", err
	}

	if err := writeFile(filepath.Join(path, "pids.max"), "64"); err != nil {
		os.Remove(path)
		return "", err
	}
	if err := writeFile(filepath.Join(path, "memory.max"), "134217728"); err != nil {
		os.Remove(path)
		return "", err
	}
	if err := writeFile(filepath.Join(path, "memory.swap.max"), "0"); err != nil {
		os.Remove(path)
		return "", err
	}

	return path, nil
}

func delegatedCgroupFsPath() string {
	return delegatedCgroupFsPathForUID(os.Getuid())
}

func delegatedCgroupFsPathForUID(uid int) string {
	return filepath.Join(cgroupRoot, cgroupName, strconv.Itoa(uid))
}

func delegatedCgroupRelPath() string {
	return "/" + cgroupName + "/" + strconv.Itoa(os.Getuid())
}

func delegatedUser() (int, int) {
	uid := os.Getuid()
	gid := os.Getgid()

	if sudoUID := os.Getenv("SUDO_UID"); sudoUID != "" {
		uid = mustAtoi(sudoUID)
	}
	if sudoGID := os.Getenv("SUDO_GID"); sudoGID != "" {
		gid = mustAtoi(sudoGID)
	}

	return uid, gid
}

func printStartError(err error) {
	fmt.Fprintf(os.Stderr, "failed to start rootless child: %v\n", err)
	if !isPermissionError(err) {
		return
	}

	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "This failed before chroot/rootfs setup, so it is not caused by ubuntu-2404-rootfs ownership.")
	fmt.Fprintln(os.Stderr, "The host is likely blocking unprivileged user namespaces, or a container/seccomp/AppArmor policy is denying CLONE_NEWUSER.")
	fmt.Fprintln(os.Stderr, "Check with: go run main.go check-rootless")
}

func printCgroupMoveError(path string, err error) {
	fmt.Fprintf(os.Stderr, "failed to move process into cgroup %s: %v\n", path, err)
	if !isPermissionError(err) {
		return
	}

	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "In cgroup v2, moving a process needs permission on the destination cgroup and on the common ancestor with the source cgroup.")
	fmt.Fprintln(os.Stderr, "Your shell is usually outside /sys/fs/cgroup/go-container-scratch/<uid>, so run through enter-cgroup first.")
	fmt.Fprintln(os.Stderr, "Example:")
	fmt.Fprintln(os.Stderr, "  go build -o go-container")
	fmt.Fprintln(os.Stderr, "  sudo ./go-container enter-cgroup ./go-container run ./ubuntu-2404-rootfs /bin/bash")
}

func isPermissionError(err error) bool {
	if pathErr, ok := err.(*os.PathError); ok {
		return pathErr.Err == syscall.EPERM || pathErr.Err == syscall.EACCES
	}
	return false
}

func printKernelValue(path string) {
	value, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("%s: unavailable (%v)\n", path, err)
		return
	}
	fmt.Printf("%s: %s", path, string(value))
}

func printStatusValue(key string) {
	status, err := os.ReadFile("/proc/self/status")
	if err != nil {
		fmt.Printf("/proc/self/status %s: unavailable (%v)\n", key, err)
		return
	}

	prefix := key + ":"
	for _, line := range strings.Split(string(status), "\n") {
		if strings.HasPrefix(line, prefix) {
			fmt.Printf("/proc/self/status %s\n", line)
			return
		}
	}
	fmt.Printf("/proc/self/status %s: unavailable\n", key)
}

func printCurrentCgroup() {
	path, err := currentCgroupPath()
	if err != nil {
		fmt.Printf("/proc/self/cgroup: unavailable (%v)\n", err)
		return
	}
	fmt.Printf("/proc/self/cgroup: %s\n", path)
}

func isInDelegatedCgroup() bool {
	return isInDelegatedCgroupForUID(os.Getuid())
}

func isInDelegatedCgroupForUID(uid int) bool {
	current, err := currentCgroupPath()
	if err != nil {
		return false
	}

	delegated := delegatedCgroupRelPathForUID(uid)
	return current == delegated || strings.HasPrefix(current, delegated+"/")
}

func delegatedCgroupRelPathForUID(uid int) string {
	return "/" + cgroupName + "/" + strconv.Itoa(uid)
}

func currentCgroupPath() (string, error) {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) == 3 && parts[0] == "0" {
			return parts[2], nil
		}
	}

	return "", fmt.Errorf("cgroup v2 entry not found")
}

func enableControllers(path string, controllers ...string) error {
	availableBytes, err := os.ReadFile(filepath.Join(path, "cgroup.controllers"))
	if err != nil {
		return err
	}

	available := strings.Fields(string(availableBytes))
	for _, controller := range controllers {
		if !contains(available, controller) {
			continue
		}
		if err := writeFile(filepath.Join(path, "cgroup.subtree_control"), "+"+controller); err != nil {
			return fmt.Errorf("enable %s in %s: %w", controller, path, err)
		}
	}

	return nil
}

func chownCgroup(path string, uid int, gid int) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	if err := os.Chown(path, uid, gid); err != nil {
		return err
	}
	for _, entry := range entries {
		if err := os.Chown(filepath.Join(path, entry.Name()), uid, gid); err != nil {
			return err
		}
	}

	return nil
}

func writeFile(path string, data string) error {
	return os.WriteFile(path, []byte(data), 0644)
}

func contains(items []string, item string) bool {
	for _, current := range items {
		if current == item {
			return true
		}
	}
	return false
}

func mustAtoi(value string) int {
	number, err := strconv.Atoi(value)
	must(err)
	return number
}
