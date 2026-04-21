package main

import (
  "fmt"
  "os"
  "os/exec"
  "syscall"
  "path/filepath"
  "io/ioutil"
  "strconv"
)

func must(err error) {
    if err != nil {
        panic(err)
    }
}

func main() {
    switch os.Args[1] {
    case "run":
        run()
    case "child":
        child()
    default:
        panic("help")
    }
}

func run() {
    fmt.Printf("Running %v\n", os.Args[2:])

    // Run child
    cmd := exec.Command("/proc/self/exe", append([]string{"child"}, os.Args[2:]...)...)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWUSER,
        Unshareflags: syscall.CLONE_NEWNS,
        Credential: &syscall.Credential{Uid: 0, Gid: 0},
        UidMappings: []syscall.SysProcIDMap{
            {ContainerID: 0, HostID: os.Getuid(), Size: 1},
        },
        GidMappings: []syscall.SysProcIDMap{
            {ContainerID: 0, HostID: os.Getgid(), Size: 1},
        },
    }

    must(cmd.Run())
}

func child() {
    fmt.Printf("Child Running %v\n", os.Args[2:])

    // Applying cgroups v2
    // cgv2()
    // comentado porque requiere root para cgroups y actualmente esta con fake root privileged

    cmd := exec.Command(os.Args[2], os.Args[3:]...)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    // Cambiar hostname
    must(syscall.Sethostname([]byte("mario21ic-container")))

    // must(syscall.Mount("", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, "")) // old way to avoid mount propagation
    must(syscall.Chroot("./ubuntu-2404-rootfs"))
    must(os.Chdir("/"))

    // Mount proc
    must(syscall.Mount("proc", "proc", "proc", 0, ""))

    // Mount tmpfs
    must(syscall.Mount("mario", "mario21ic_temp", "tmpfs", 0, ""))

    // Running
    must(cmd.Run())

    // Umount
    must(syscall.Unmount("proc", 0))
    must(syscall.Unmount("mario21ic_temp", 0)) // unmount tmpfs
}

func cgv2() {
    cgroups := "/sys/fs/cgroup/"
    path := filepath.Join(cgroups, "mario21ic")
    os.Mkdir(path, 0755)

    // Set memory limitation
    must(ioutil.WriteFile(filepath.Join(path, "memory.max"), []byte("8097152"), 0700))
    // Disable swap
    must(ioutil.WriteFile(filepath.Join(path, "memory.swap.max"), []byte("0"), 0700))

    // Add the current process (or any PID) to the cgroup
    pid := strconv.Itoa(os.Getpid())
    must(ioutil.WriteFile(filepath.Join(path, "cgroup.procs"), []byte(pid), 0700))
}
