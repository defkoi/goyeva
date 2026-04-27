function createLogo(
  container = document.body,
  {
    size = 400,
    backgroundColor = "black",
    textColor = "white",
    text = "yv",
    fontFamily = "cambria",
    className = "",
    id = "",
  },
) {
  const backDiv = document.createElement("div");
  backDiv.className = className;
  if (id) backDiv.id = id;

  Object.assign(backDiv.style, {
    backgroundColor,
    width: `${size}px`,
    height: `${size}px`,
    display: "flex",
    justifyContent: "end",
    alignItems: "end",
  });

  const textDiv = document.createElement("div");
  textDiv.textContent = text;

  const innerSize = Math.floor(size * 0.8);
  const fontSize = Math.floor(innerSize * 0.8);

  Object.assign(textDiv.style, {
    color: textColor,
    width: `${innerSize}px`,
    height: `${innerSize}px`,
    display: "flex",
    justifyContent: "center",
    alignItems: "center",
    fontFamily,
    fontSize: `${fontSize}px`,
  });

  backDiv.appendChild(textDiv);
  container.appendChild(backDiv);

  return backDiv;
}

document.addEventListener("DOMContentLoaded", () => {
  createLogo(document.body, { text: "YV", backgroundColor: "#1c8ca8" });
});
