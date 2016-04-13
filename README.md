# hipchat
[![](https://circleci.com/gh/Bowbaq/hipchat/tree/master.svg?&style=shield&circle-token=f1e69183a5aababcc75d5313890189ce6e5a1e2b)](https://circleci.com/gh/Bowbaq/hipchat/tree/master)

Hipchat lets you download an archive of your private (1-1) hipchat conversations

## Installation

### Quick Installation

**Note:** this is the recommended method if you don't plan to look at the code / do any development on the tool.

1. Open a Terminal
2. Paste the command below:
   `curl -fsSL https://raw.githubusercontent.com/Bowbaq/hipchat/master/install | bash`
3. After a few seconds, `Installation complete` should appear in your Terminal window

### Manual Steps

#### Using the go tools

If you have `go` installed, a simple `go get github.com/Bowbaq/hipchat` should do the trick.

#### Downloading the lastest release

Head to the [release section](https://github.com/Bowbaq/hipchat/releases) and download the binary
for your machine, then place it in your `$PATH`.

## Usage

In order to get your private messages from HipChat you'll need to obtain an API token.

1. Install the tool
2. In your browser, go to [https://www.hipchat.com/account/api](https://www.hipchat.com/account/api). You'll
   need to log in, then enter your password one more time.
3. Create a token with the `view_group` and `view_messages` scopes.
4. In the Terminal, paste the following command, replacing `<api token>` with the token generated in step 3.
   `/usr/local/hipchat dump -t <api token> -d "$HOME/Documents/hipchat-messages.zip"`
