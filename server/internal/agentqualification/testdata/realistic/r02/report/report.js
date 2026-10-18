export function canonicalTotal(data) {
  // Intentional fixture defect: joined metric rows can duplicate a post.
  return data.metric_rows.reduce((sum, row) => sum + row.views, 0);
}

export function renderReport(data) {
  document.querySelector("#total").textContent = canonicalTotal(data);
  document.querySelector("#cards").innerHTML = data.published_posts
    .slice(0, 3)
    .map((post) => `<article class="card"><h2>${post.id}</h2><p>${post.views} views</p></article>`)
    .join("");
}

if (typeof document !== "undefined") {
  fetch("./data.json").then((response) => response.json()).then(renderReport);
}
