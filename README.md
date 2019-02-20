# MTR
[![Build Status](https://travis-ci.org/tonobo/mtr.svg?branch=master)](https://travis-ci.org/tonobo/mtr)

A MTR implementation written in golang, completely without shell-execs.

Just install with the following command:
```
go get -u github.com/tonobo/mtr
sudo setcap cap_net_raw+ep PATH-TO-GOMTR
```

**Note: This tool maybe called with sudo or as root, because it requires raw sockets.**

## Output

*Currently there is no support for non ipv4 targets*

```
HOP:    Address                Loss%  Sent    Last     Avg    Best   Worst       Packets
  1:|-- 10.200.1.1              0.0%     6     1.4     1.8     1.2     3.2         .....
  2:|-- 192.168.2.1             0.0%     6     3.3     3.1     2.4     3.5         .....
  3:|-- 62.155.247.163          0.0%     6    10.9    11.6    10.9    12.6         .....
  4:|-- 217.5.95.170            0.0%     6    24.5    24.9    24.5    26.0         .....
  5:|-- 72.14.194.156           0.0%     6    23.9    25.1    23.2    30.4         .....
  6:|-- ???                   100.0%     6     0.0     0.0     0.0     0.0         ?????
  7:|-- 209.85.142.128          0.0%     6    24.5    24.9    23.6    25.8         .....
  8:|-- 108.170.227.227         0.0%     6    25.4    25.6    24.0    30.0         .....
  9:|-- 8.8.8.8                 0.0%     5    23.9    24.2    23.6    25.0          ....
```

**JSON:**

```json
{
  "destination": "8.8.8.8",
  "statistic": {
    "1": {
      "avg_ms": 1.3946096666666667,
      "best_ms": 1.3157679999999998,
      "last_ms": 1.322044,
      "loss_percent": 0,
      "packet_buffer_size": 10,
      "packet_list_ms": [
        {
          "respond_ms": 1.322044,
          "success": true
        },
        {
          "respond_ms": 1.3157679999999998,
          "success": true
        },
        {
          "respond_ms": 1.546017,
          "success": true
        },
        null,
        null,
        null,
        null,
        null,
        null,
        null
      ],
      "sent": 3,
      "target": "10.200.1.1",
      "ttl": 1,
      "worst_ms": 1.546017
    },
    "2": {
      // ...
    }
  }
}
```

## Usage

```
Usage:
  mtr TARGET [flags]

Flags:
      --buffer-size int     Cached packet buffer size (default 50)
  -c, --count int           Amount of pings per target (default 5)
  -h, --help                help for mtr
  -i, --interval duration   Wait time between icmp packets before sending new one (default 100ms)
      --json                Print json results
      --max-hops int        Maximal TTL count (default 64)
  -t, --timeout duration    ICMP reply timeout (default 800ms)
```
