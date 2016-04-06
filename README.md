### simple redis protocol server


```
package main

import (
	"github.com/orca-clock/resp"
)

func main() {
	resp.AddHandler("ping", func(conn *resp.Conn, req *resp.Request) {
		conn.WriteStatus("pong")
	})

	resp.ListenAndServe(":6000")
}

```
### 输出
```
127.0.0.1:6000> ping
pong
127.0.0.1:6000> get
(error) unsurport method:get
127.0.0.1:6000> 
```
