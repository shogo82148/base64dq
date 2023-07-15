# base64dq

Package base64dq implements a base64 encoding variant that is inspired by the Revival Password of Dragon Quest.

## Revival Password (ふっかつのじゅもん)

The Revival Password (ふっかつのじゅもん) is a string of 20 characters that is used to revive a player's party in the [Dragon Quest] series.
It is encoded in a custom base64 variant that uses 64 characters from the Japanese hiragana syllabary.

[Dragon Quest]: https://www.dragonquest.jp/

## base64dq

base64 uses the Japanese hiragana instead of ascii alphabets and digits.

| Value | base64 Alphabet | base64dq Alphabet |
| ----- | --------------- | ----------------- |
| 0     | A               | あ                |
| 1     | B               | い                |
| 2     | C               | う                |
| 3     | D               | え                |
| 4     | E               | お                |
| 5     | F               | か                |
| 6     | G               | き                |
| 7     | H               | く                |
| 8     | I               | け                |
| 9     | J               | こ                |
| 10    | K               | さ                |
| 11    | L               | し                |
| 12    | M               | す                |
| 13    | N               | せ                |
| 14    | O               | そ                |
| 15    | P               | た                |
| 16    | Q               | ち                |
| 17    | R               | つ                |
| 18    | S               | て                |
| 19    | T               | と                |
| 20    | U               | な                |
| 21    | V               | に                |
| 22    | W               | ぬ                |
| 23    | X               | ね                |
| 24    | Y               | の                |
| 25    | Z               | は                |
| 26    | a               | ひ                |
| 27    | b               | ふ                |
| 28    | c               | へ                |
| 29    | d               | ほ                |
| 30    | e               | ま                |
| 31    | f               | み                |
| 32    | g               | む                |
| 33    | h               | め                |
| 34    | i               | も                |
| 35    | j               | や                |
| 36    | k               | ゆ                |
| 37    | l               | よ                |
| 38    | m               | ら                |
| 39    | n               | り                |
| 40    | o               | る                |
| 41    | p               | れ                |
| 42    | q               | ろ                |
| 43    | r               | わ                |
| 44    | s               | が                |
| 45    | t               | ぎ                |
| 46    | u               | ぐ                |
| 47    | v               | げ                |
| 48    | w               | ご                |
| 49    | x               | ざ                |
| 50    | y               | じ                |
| 51    | z               | ず                |
| 52    | 0               | ぜ                |
| 53    | 1               | ぞ                |
| 54    | 2               | だ                |
| 55    | 3               | ぢ                |
| 56    | 4               | づ                |
| 57    | 5               | で                |
| 58    | 6               | ど                |
| 59    | 7               | ば                |
| 60    | 8               | び                |
| 61    | 9               | ぶ                |
| 62    | +               | べ                |
| 63    | /               | ぼ                |
| (pad) | =               | ・                |
