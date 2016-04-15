# hipchat
[![](https://circleci.com/gh/Bowbaq/hipchat/tree/master.svg?&style=shield&circle-token=f1e69183a5aababcc75d5313890189ce6e5a1e2b)](https://circleci.com/gh/Bowbaq/hipchat/tree/master)

Hipchat lets you download an archive of your private (1-1) hipchat conversations

## Installation

1. Open a Terminal
2. Paste the command below:

   `curl -fsSL https://raw.githubusercontent.com/Bowbaq/hipchat/master/install | bash`
3. After a few seconds, `Installation complete` should appear in your Terminal window. Keep that window open,
   you'll need it to download your messages.

   ![Installation](/imgs/installation.png?raw=true "Installation")

*Note: If you don't use OSX/Linux, head to the [release section](https://github.com/Bowbaq/hipchat/releases)
and download the binary for your machine, then place it in your `$PATH`.*

## Usage

In order to get your private messages from HipChat you'll first need to obtain an API token.

1. Install the tool by following the quick installation [steps]((#quick-installation))
2. In your browser, go to [https://www.hipchat.com/account/api](https://www.hipchat.com/account/api). You'll
   need to log in, then enter your password one more time.
3. Create a token with the `view_group` and `view_messages` scopes.

   ![Token Creation](/imgs/create-token.png?raw=true "Token Creation")
4. You should now see your API token

   ![Token Created](/imgs/token-created.png?raw=true "Token Created")
5. In the Terminal, paste the following command, replacing `<api token>` with the token from step 4.

   `~/bin/hipchat-archive -f "$HOME/Documents/hipchat-messages.zip" -t <api token>`

   This might take a while. If it looks like it's hanging, just wait a few minutes, the API rate limits are low.

   ![Usage](/imgs/usage.png?raw=true "Usage")

6. You're done. To browse your archive, you can run `open "$HOME/Documents/hipchat-messages.zip"` in the Terminal.

## Development

If you have `go` installed, a simple `go get github.com/Bowbaq/hipchat` should do the trick.
