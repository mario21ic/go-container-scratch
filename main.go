package main

import (
  "fmt"
  "os"
  "os/exec"
  "syscall"
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
        Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
        Unshareflags: syscall.CLONE_NEWNS,
    }

    must(cmd.Run())
}

func child() {
    fmt.Printf("Child Running %v\n", os.Args[2:])

    cmd := exec.Command(os.Args[2], os.Args[3:]...)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    // Cambiar hostname
    must(syscall.Sethostname([]byte("mario21ic-container")))

    // must(syscall.Mount("", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, "")) // old way to avoid mount propagation
    must(syscall.Chroot("/home/ubuntu/repo/container-scratch/ubuntu-2404-rootfs"))
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
