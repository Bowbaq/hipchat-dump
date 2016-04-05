# hipchat-dump
[![](https://circleci.com/gh/Bowbaq/hipchat/tree/master.svg?&style=shield&circle-token=f1e69183a5aababcc75d5313890189ce6e5a1e2b)](https://circleci.com/gh/Bowbaq/hipchat/tree/master)

Quick tool to get a dump of private (1-1) hipchat conversations

## Installation

If you have `go` installed, a simple `go get github.com/Bowbaq/hipchat` should do the trick.

If not, head to the [release section](https://github.com/Bowbaq/hipchat/releases) and download the binary
for your machine. Some common platforms:

| Platform    |                 Binary name |
| ----------- | --------------------------: |
| **OSX**     |      `hipchat_darwin_amd64` |
| **Linux**   |       `hipchat_linux_amd64` |

Usage: `hipchat dump -t <your api token>`

You can get an API token [here](https://www.hipchat.com/account/api). It must have at least the `view_group`
and `view_messages` groups.
