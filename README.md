# beflash - Parallel Behat feature runner

Runs **Behat** features files in parallel. Note: this requires your test suite to be compatible with parallel
runner. For example in our case, we isolate scenarios within a database transaction and this allows us to run
tests in parallel. When scenario ends - transaction is rolled back and the state is fresh again.

## The success story

We had our test suite running for **15** minutes, now we run it within **1.5** minute. It tests the same **MySQL**
database with all the constraints and state cleanup, including **symfony2** kernel reboot on each scenario.

So in our case:
- we run our tests **10x** faster, our **CI** is happy.
- we have found other application bugs, like doctrine ORM collection handling, database transaction deadlocks..

## Under the hood

The runner, based on concurrency level, executes few behat feature files and handles the output stream.

## Installation

To install, simply run:

    go get github.com/DATA-DOG/beflash

It should be installed in your **$GOPATH/bin** and since it is probably in your **$PATH**,
run it in your project directory by calling `beflash`

It expects to find behat executable at `./bin/behat` and scans `features/` directory for scenarios to run at the moment.

## Concurrency level

By default it uses the available number of CPUs, to modify:

    beflash -c 2

## Command options

    beflash -h

## Final words

It is still work in progress - use it at your own discretion!

## TODO

- show failures the same way as default behat runner does
- create a demo project to demonstrate the behavior
- bug: when beflash fails to open one of the feature files, it just skips it for whole suite. (Unknown error while proccessing output: read |0: bad file descriptor)

