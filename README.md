# beacon
Random Number Beacon

Based on [NIST's new beacon prototype](https://beacon.nist.gov/home).

The idea is to randomly generate bits at specific intervals
to allow for two parties which don't trust each other to agree
on a shared random number. For example, they might agree
to use the next generated number from a third party beacon.

"Beacon-bits" are signed, and hashed together to make it
more difficult for anyone (including the source) to tamper
with the bits.
