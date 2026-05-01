Based on https://baconyao.notion.site/Containers-From-Scratch-by-Golang-feat-Liz-Rice-2938a3a7d9d480dc9598e8efd86cfd4b#2938a3a7d9d480fcba55e8692eabd2dc


Init:
```
./init.sh
```

Setup delegated cgroup once as root:
```
sudo go run main.go setup-cgroup
```

Run rootless with the delegated cgroup:
```
go build -o go-container
sudo ./go-container enter-cgroup ./go-container run ./ubuntu-2404-rootfs /bin/bash
```

If rootless startup fails with `operation not permitted`, check the host:
```
go run main.go check-rootless
```

Try Linux native cli:
```
sudo unshare --pid --uts --net --mount --fork --mount-proc bash
sudo lsns
```

TODO:
* Conexion a internet
* Compartir namespace como network similar a Pods
* Permitir overlay2 como file system
* Hacer bind mount de rutas especificas
* Guardar images
