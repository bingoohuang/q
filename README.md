# q

q is a better way to do print statement debugging.

Type `q.Q` instead of `fmt.Printf` and your variables will be printed like this:

![q output examples](https://i.imgur.com/OFmm7pb.png)

## Why is this better than `fmt.Printf`?

* Faster to type
* Pretty-printed vars and expressions
* Easier to see inside structs
* Doesn't go to noisy-ass stdout. It goes to `$TMPDIR/$USER/q`.
* Pretty colors!

## Basic Usage

```go
import "q"
...
q.Q(a, b, c)
```

For best results, dedicate a terminal to tailing `$TMPDIR/$USER/q` while you work.

## Install

`git get  github.com/bingoohuang/q@latest`

Put these functions in your shell config. Typing `qq` or `rmqq` will then start
tailing `$TMPDIR/$USER/q`.

```sh
qq() {
    clear

    logpath="$TMPDIR/$USER/q"
    if [[ -z "$TMPDIR" ]]; then
        logpath="/tmp/q"
    fi

    if [[ ! -f "$logpath" ]]; then
        echo 'Q LOG' > "$logpath"
    fi

    tail -100f -- "$logpath"
}

rmqq() {
    logpath="$TMPDIR/$USER/q"
    if [[ -z "$TMPDIR" ]]; then
        logpath="/tmp/q"
    fi
    if [[ -f "$logpath" ]]; then
        rm "$logpath"
    fi
    qq
}
```

You also can simply `tail -f $TMPDIR/$USER/q`, but it's highly recommended to use the above commands.

## Haven't I seen this somewhere before?

Python programmers will recognize this as a Golang port of
the [`q` module by zestyping](https://github.com/zestyping/q).

Ping does a great job of explaining `q` in his awesome lightning talk from
PyCon 2013. Watch it! It's funny :)

[![ping's PyCon 2013 lightning talk](https://i.imgur.com/7KmWvtG.jpg)](https://youtu.be/OL3De8BAhME?t=25m14s)

## FAQ

### Why `q.Q`?

It's quick to type and unlikely to cause naming collisions.

### Is `q.Q()` safe for concurrent use?

Yes.
