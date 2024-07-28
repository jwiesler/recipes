document.addEventListener("DOMContentLoaded", function () {
    const categoriesOutput = document.getElementById("categories");
    const recipes = document.querySelectorAll(".recipes-list > li");
    const categories = new Set();
    recipes.forEach(e => {
        const attr = e.getAttribute("data-categories");
        const cs = attr == null || attr.length === 0 ? [] : attr.split(",");
        e.categories = cs;
        if (attr != null && attr.length !== 0) {
            cs.forEach(c => categories.add(c));
        }
    });

    const inactive = "text-bg-secondary";
    const active = "text-bg-primary";
    categories.forEach(c => {
        const div = document.createElement("div");
        div.classList.add("badge", inactive, "user-select-none");
        div.setAttribute("role", "button");
        div.innerText = c;
        categoriesOutput.appendChild(div);
        div.addEventListener("click", () => {
            let filter;
            if (div.classList.contains(inactive)) {
                categoriesOutput.childNodes.forEach(c => {
                    c.classList.remove(active);
                    c.classList.add(inactive);
                });
                div.classList.remove(inactive);
                div.classList.add(active);
                filter = div.innerText;
            } else {
                div.classList.remove(active);
                div.classList.add(inactive);
                filter = null;
            }
            recipes.forEach(c => {
                if (filter == null || c.categories.indexOf(filter) !== -1) {
                    c.style.removeProperty("display");
                } else {
                    c.style.display = "none";
                }
            });
        });
    });
});
