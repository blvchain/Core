---
globs: protos/*
description: Apply when modifying API surface or adding fields like contract
  upload metadata.
---

If request handling logic depends on new fields in the proto messages, update protos/gate.proto and regenerate Go protobufs (protoc) accordingly; include language and filename when showing code blocks.