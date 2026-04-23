![Yeva](./ext/images/logo.png)

```javascript
struct Vector {
  .new(x: 0, y: 0) {
    if !(typeof x == "number" && typeof y == "number")
      throw "numbers expected"
    return struct (Vector) { .x, .y }
  },
  ->add(destruct { .x: 0, .y: 0 }) => Vector.new(
      self.x + x,
      self.y + y,
    ),
}

var vec1 = Vector.new(1, 2)
var vec2 = Vector.new(3, 4)
var vec3 = vec1->add(vec2)


print("x =", vec3.x) /* x = 4 */
print("y =", vec3.y) /* y = 6 */

/* or */

for (var destruct { .key, .value } in pairs(vec3)) {
  print(key, "=", value)
}
```
