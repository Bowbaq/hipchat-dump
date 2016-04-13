# hipchat
[![](https://circleci.com/gh/Bowbaq/hipchat/tree/master.svg?&style=shield&circle-token=f1e69183a5aababcc75d5313890189ce6e5a1e2b)](https://circleci.com/gh/Bowbaq/hipchat/tree/master)

Hipchat lets you download an archive of your private (1-1) hipchat conversations

## Installation

### Quick Installation

**Note:** this is the recommended method if you don't plan to look at the code / do any development on the tool.

1. Open a Terminal
2. Paste the command below:

   `curl -fsSL https://raw.githubusercontent.com/Bowbaq/hipchat/master/install | bash`
3. After a few seconds, `Installation complete` should appear in your Terminal window. Keep that window open,
   you'll need it to download your messages.

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

   ![Token Creation](/imgs/create-token.png?raw=true "Token Creation")
4. Copy the generated token in your clipboard

   ![Token Created](/imgs/token-created.png?raw=true "Token Created")
5. In the Terminal, paste the following command, replacing `<api token>` with the token you copied in step 4.
   `/usr/local/hipchat dump -t <api token> -f "$HOME/Documents/hipchat-messages.zip"`
