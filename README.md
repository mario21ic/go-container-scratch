Based on https://baconyao.notion.site/Containers-From-Scratch-by-Golang-feat-Liz-Rice-2938a3a7d9d480dc9598e8efd86cfd4b#2938a3a7d9d480fcba55e8692eabd2dc


Init:
```
./init.sh
```

Run without cgroups:
```
go run main.go run /bin/bash
```

Run with cgroups:
Uncomment line 57, then:
```
sudo go run main.go run /bin/bash
```

Try Linux native cli:
```
sudo unshare --pid --uts --net --mount --fork --mount-proc bash
sudo lsns
```
