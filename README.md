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

# Sample Server

API for the sample server:

Note: all calls are HTTP GET requests.

## /

Returns the latest bits

## /raw

Returns the raw audio (no headers) used to generate the last bits
to verify the bits were generated correctly. Only the most recent
audio sample is stored on the 

## /audio

Same as `/raw` except has properly formatted audio header (top 44 bytes)
so you can listen to it with any reasonable audio player and decide if
the noise is suitably random to your preferences.

## /key

The public key used to sign bits.

## /:id

Returns the bits with specified id.

## /before/:time

Returns the bits closest to, but before, the given time. Time is unix time,
so number of seconds since Wed Dec 31 1969 16:00:00 GMT-0800 (Pacific Standard Time)

## /after/:time

Same as before but closest time after the given time.
