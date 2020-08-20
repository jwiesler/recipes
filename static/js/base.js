function extendSection(section) {
    section.ingredientsTable = $(section).children(".ingredients-table").first()[0]
    section.ingredients = function() {
        return this.ingredientsTable.childNodes
    }
    section.headingInput = $(section).find(".ingredients-section-name-input")[0]
}

function doRedirect(xhr) {
    console.assert(xhr.responseURL)
    if(xhr.responseURL)
        window.location.href = xhr.responseURL
}

function XHRResultHandler(xhr, success, failure) {
    return function() {
        if(xhr.readyState !== 4)
            return
        if(xhr.status !== 200) {
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
    if(!a.previousSibling)
        return
    a.parentNode.insertBefore(a, a.previousSibling)
}

function moveDown(a) {
    if(!a.nextSibling)
        return
    a.parentNode.insertBefore(a.nextSibling, a)
}

function createInitialState() {
    return {
        ingredients: $("#ingredients-sections")[0],
        description: $("#description")[0],
        instructions: $("#instructions")[0],
        title: $("#name")[0],
        source: $("#source")[0],
        findSections: function() {
            return $(this.ingredients).children("div")
        }
    }
}
