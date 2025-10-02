// example of blvchain vm usage

// variables
var a = 10
var b = [1, 2, 3]
var c = {"a": 1, "b": 2}

// operator
All supported operators:
	+ - * / % ^ == != < <= > >= && ||

// functions
Function are two types:
	1. Built-in functions
	2. User-defined functions

Built-in functions:
	D256(str, step, repeat)
	D512(str, step, repeat)
	D256C(str, path)
	D512C(str, path)
  VerifySignature(pubkey, uid, message, signature)
  MakeUID(pubkey)
  GetOneBlockDataWithBlockHash(blockHash)
  len(arr)
  getFromArrWithIndex(arr, index)
  getFromObjWithKey(obj, key)

User-defined functions:
  You can define your own functions in the same way as built-in functions.

Conditions:
  You can use the following conditions:
    1. ==
    2. !=
    3. <
    4. >
    5. <=
    6. >=
    7. &&
    8. ||
    9. !

  You can use the following operators:
    1. +
    2. -
    3. *
    4. /
    5. ^
    6. %

  You can use the following statements:
    1. var
    2. if
    3. for
    4. return

  You can use the following types:
    1. array
    2. string
    3. int
    4. object
    5. bool

example of conditionals:
  var a = 10
  if a == 10 {
    return "a is 10"
  } else {
    return "a is not 10"
  }

example of operators:
  var a = 10
  var b = a + 5
  var c = b * 2
  var d = c / 2
  var e = d ^ 2
  var f = e % 2

example of statements:
  var a = 10
  var b = a + 5
  var c = b * 2
  var d = c / 2
  var e = d ^ 2
  var f = e % 2

example of types:
  var a = 10
  var b = [1, 2, 3]
  var c = {"a": 1, "b": 2}
  var d = true

