# zedcache

A transparent cache for [authzed](https://authzed.com)'s zedtokens.

## Motivation

To boost performance while still avoiding security pitfalls like the [New Enemy Problem](https://authzed.com/docs/reference/glossary#new-enemy-problem) [authzed](https://authzed.com) requires the use of [zedtokens](https://authzed.com/docs/reference/zedtokens-and-zookies).
zedtokens are supposed to be saved along the resource itself, forcing developers to create new database rows.
zedcache tries to transparently cache zedtokens and boost performance.

## Cache

The kind of cache used is crucial.
An inconsistent cache can lead to the [New Enemy Problem](https://authzed.com/docs/reference/glossary#new-enemy-problem).
