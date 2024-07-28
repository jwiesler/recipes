const errors = {
    "already-exists": "Ein Rezept mit diesem Namen existiert bereits.",
    "unauthorized": "Zugriff verweigert.",
    "empty-id": "Ein Rezept muss einen Namen haben.",
    "internal-error": "Internal server error.",
    "user-name-too-short": "Username ist zu kurz, mindestens 4 Zeichen.",
    "password-too-short": "Passwort ist zu kurz, mindestens 8 Zeichen.",
}

function extendSection(section) {
    section.ingredientsTable = section.querySelector(".ingredients-table")
    section.ingredients = function () {
        return this.ingredientsTable.childNodes
    }
    section.headingInput = section.querySelector(".ingredients-section-name-input")
}

function doRedirect(xhr) {
    console.assert(xhr.responseURL)
    if (xhr.responseURL)
        window.location.href = xhr.responseURL
}

function XHRResultHandler(xhr, success, failure) {
    return function () {
        if (xhr.readyState !== 4)
            return
        if (xhr.status !== 200) {
            failure(xhr)
        } else {
            success(xhr)
        }
    }
}

function removeNode(n) {
    n.parentNode.removeChild(n)
}

function moveUp(a) {
    if (!a.previousSibling)
        return
    a.parentNode.insertBefore(a, a.previousSibling)
}

function moveDown(a) {
    if (!a.nextSibling)
        return
    a.parentNode.insertBefore(a.nextSibling, a)
}

function createInitialState() {
    return {
        ingredients: document.getElementById("ingredients"),
        ingredientsSections: document.getElementById("ingredients-sections"),
        description: document.getElementById("description"),
        instructions: document.getElementById("instructions"),
        title: document.getElementById("name"),
        source: document.getElementById("source"),
        categories: document.getElementById("categories"),
        findSections: function () {
            return this.ingredientsSections.querySelectorAll(":scope > div")
        }
    }
}
