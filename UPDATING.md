# Updating from upstream

We'll use the upstream version v1.14.10 as an example.

## Add origin

```
git add remote upstream git@github.com:ethereum/go-ethereum.git
```

## Find version to upgrade to

Go to https://github.com/ethereum/go-ethereum/releases and find the tag you want to update to

## Check out latest bn/geth and upstream/geth

```
git fetch origin
git fetch upstream
git checkout mempool-feed-stage
git pull origin mempool-feed-stage
```

## Create working branch

```
git checkout -b upgrade/v1.14.10
```

## Merge

```
git merge v1.14.10
```

## Fix conflicts

No systemic way to handle this. Use `git status` and `git diff` to find the conflicts and compare against upstream.

`git commit` to finish after all conflicts are fixed.

## Check

`make geth` to build and ensure there are no compilation errors. Fix any issues and repeat until it builds.

## Push / test

Push the branch to Github and make a Pull Request. Let the tests run. Fix any issues and repeat until they pass.

## Stream API check


###
SSH into a staging geth node with 8546 forwarded to your local machein.

Example: `ssh -L 8546:0.0.0.0:8546 10.0.1.29`

### Update Geth

```
sudo su ubuntu
cd ~/go-ethereum
git fetch origin
git checkout origin/upgrade/v1.14.10
export PATH=/opt/go/1.21.0/bin:$PATH
make geth
sudo systemctl stop geth && sudo cp ./build/bin/geth /usr/local/bin/geth && sudo systemctl start geth
```

Make sure it's running with `sudo journalctl -u geth.service -f`

###
Connect to the trace stream from your local machine using wscat:

```
wscat -w 10 -c http://localhost:8546 -x '{"id":1,"jsonrpc":"2.0","method":"eth_subscribe","params":["newPendingTransactionsWithTrace"]}'
```

## Merge Pull Request + Tag + Release

Now that all testing is done you can merge and close the Pull Request. Now pull the latest `mempool-feed-stage` and tag it with the upstream tag plus `-1`. Any updates based on this version will increment this suffix.

```
git checkout mempool-feed-stage
git fetch origin
git pull origin mempool-feed-stage
git tag v1.14.10-1
git push origin v1.14.10-1
```

Now to go https://github.com/blocknative/go-ethereum/releases and create a new release with the tag.