function createInitialState(root) {
    return {
        ingredients: root.find(".ingredients-sections").first(),
        descriptionInput: root.find("#description")[0],
        instructionsInput: root.find("#instructions")[0],
        titleInput: root.find("#name")[0],
        sourceInput: root.find("#source")[0],
        findSections: function() {
            return this.ingredients.children("div")
        }
    }
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

function getIngredientsTable(section) {
    return section.children(".ingredients-table").first()
}

function getIngredientsOfSection(section) {
    return getIngredientsTable(section).children()
}

function removeNode(n) {
    n.parentNode.removeChild(n)
}

function initToolbar(node, toolbar) {
    const deleteButton = toolbar.find(".button-delete").first()
    const upButton = toolbar.find(".button-up").first()
    const downButton = toolbar.find(".button-down").first()
    deleteButton.click(function() {
        removeNode(node)
    })
    upButton.click(function() {
        moveUp(node)
    })
    downButton.click(function() {
        moveDown(node)
    })
}

function initListersForRow(row) {
    const jrow = $(row)
    initToolbar(row, jrow)
}

function initListenersForSection(section, defaultRow) {
    const jsection = $(section)
    const toolbar = jsection.find(".toolbar-wrapper")
    console.assert(toolbar.length !== 0)
    initToolbar(section, toolbar)
    getIngredientsOfSection(jsection).each(function(i, row) {
        initListersForRow(row)
    })
    const addButton = jsection.find(".button-add").first()
    const table = getIngredientsTable(jsection)[0]
    addButton.click(function() {
        const e = defaultRow.cloneNode(true)
        table.appendChild(e)
        initListersForRow(e)
    })
}

$(function() {
    const form = $("#recipe-edit-form")[0]
    console.assert(form)
    form.reset()
    const info = createInitialState($(form))
    const defaultRow = $("#default-row")[0].firstChild
    const defaultSection = $("#default-section")[0].firstChild
    console.assert(defaultRow)
    console.assert(defaultSection)

    info.findSections().each(function(i, section) {
        initListenersForSection(section, defaultRow)
    })

    const addButton = $(form).find(".button-add-section")
    addButton.click(function() {
        const e = defaultSection.cloneNode(true)
        info.ingredients[0].appendChild(e)
        initListenersForSection(e, defaultRow)
    })

    const submitToast = $("#toast-submit-failed")
    const submitToastContent = submitToast.find(".toast-body")[0]
    const submitButton = $(form).find(".submit-button")[0]
    function saveRecipe() {
        const title = info.titleInput.value
        const description = info.descriptionInput.value
        const instructions = info.instructionsInput.value
        const source = info.sourceInput.value
        const sections = info.findSections()
        const resArray = new Array(sections.length)
        const image = $("#image").attr("src")
        for (let i = 0; i < sections.length; i++) {
            const sec = $(sections[i])
            const heading = sec.find(".ingredients-section-name-input")[0].value

            const tableRows = getIngredientsOfSection(sec)
            const ingredientsArray = new Array(tableRows.length)
            for (let j = 0; j < tableRows.length; j++) {
                const row = $(tableRows[j])
                const ingredientAmount = row.find(".ingredient-amount-input")[0].value
                const ingredientAmountUnit = row.find(".ingredient-unit-input")[0].value
                const ingredientName = row.find(".ingredient-name-input")[0].value
                ingredientsArray[j] = {
                    Amount: ingredientAmount,
                    Unit: ingredientAmountUnit,
                    Name: ingredientName,
                }
            }
            resArray[i] = {
                Heading: heading,
                Ingredients: ingredientsArray,
            }
        }

        const res = {
            Name: title,
            ImagePath: image,
            Description: description,
            IngredientsSections: resArray,
            Instructions: instructions,
            Source: source,
        }
        const json = JSON.stringify(res)
        const xhr = new XMLHttpRequest()
        xhr.open(form.method, form.action, true)
        xhr.setRequestHeader("Content-Type", 'application/json; charset=UTF-8')
        xhr.onreadystatechange = function(r) {
            if(xhr.readyState !== 4)
                return
            if(xhr.status !== 200) {
                submitToastContent.innerText = "Antwort des Servers: " + xhr.responseText
                submitToast.toast("dispose")
                submitToast.toast("show")
                console.error("Failed with status code " + xhr.status + " (" + xhr.statusText + "): " + xhr.responseText)
                submitButton.disabled = false
            } else if(xhr.responseURL) {
                window.location.href = xhr.responseURL
            }

        }
        xhr.send(json)
    }

    submitButton.addEventListener("click", function(e) {
        submitButton.disabled = true
        saveRecipe()
    })
    form.addEventListener("submit", function(e) {
        e.preventDefault()
    })
})