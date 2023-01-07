Neon Modem Overdrive
--------------------

[*Neon Modem Overdrive*][neonmodem] is a BBS-like command line client that 
supports [Discourse][discourse], [Lemmy][lemmy], [Lobsters][lobsters] and 
[Hacker News][hackernews] as backends and seamlessly integrates all of them into 
a streamlined TUI. And yes, you heard that right, I really called it *Neon Modem 
Overdrive*.

*Neon Modem* is built in Go, using [Charm's Bubbletea][bubbletea] TUI framework, 
but implements an own *window manager* (or *compositor* if you want) that allows 
it to use a third dimension, on top of the two dimensional rendering that 
Bubbletea offers today. With that it is possible to display dialogs on top of 
one another, in order to offer a smoother UI experience.

[neonmodem]: https://neonmodem.com
[discourse]: https://github.com/discourse
[lemmy]: https://github.com/LemmyNet
[lobsters]: https://github.com/lobsters/lobsters
[hackernews]: https://news.ycombinator.com
[bubbletea]: https://github.com/charmbracelet/bubbletea


## Build

To build this software, simply run `make` within the cloned repository:

```sh
make
```

The binary is called `neonmodem`


## Setup

Before launching *Neon Modem Overdrive*, it requires initial setup of the 
services (a.k.a. *systems*). Run `neonmodem --help` to find out more.

