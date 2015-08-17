# beacon
Random Number Beacon

Based on [NIST's new beacon prototype](https://beacon.nist.gov/home).

The idea is to generate "random" bits at specific intervals
to allow for two parties which don't trust each other to agree
on a shared random number. For example, they might agree
to use the next generated number from a third party beacon.

"Beacon-bits" are signed, and hashed with the previous bits.
The first bits are hashed with the public key. This chains
all the bits together. All hashing is done with the base64
encoded bits so that you can copy and paste what you get
from the server to check it "by hand".
