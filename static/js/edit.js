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

function extendRow(row) {
    const jrow = $(row)
    row.ingredient = {
        nameInput: jrow.find(".ingredient-name-input")[0],
        unitInput: jrow.find(".ingredient-unit-input")[0],
        amountInput: jrow.find(".ingredient-amount-input")[0],
    }
}

function initListenersForRow(row) {
    const jrow = $(row)
    initToolbar(row, jrow)
}

let defaultRow;
let defaultSection;

function addRowToTable(table) {
    const e = defaultRow.cloneNode(true)
    table.appendChild(e)
    extendRow(e)
    initListenersForRow(e)
    return e
}

function initSection(section, defaultRow, importInformation) {
    const jsection = $(section)
    extendSection(section)
    const toolbar = jsection.find(".toolbar-wrapper")
    console.assert(toolbar.length !== 0)
    initToolbar(section, toolbar)
    toolbar.find(".button-import")[0].addEventListener("click", function() {
        importInformation.targetSection = section
        importInformation.reset()
        importInformation.modal.modal("show")
    })

    section.ingredients().forEach(function(row) {
        extendRow(row)
        initListenersForRow(row)
    })
    const addButton = jsection.find(".button-add").first()
    addButton.click(function() {
        addRowToTable(section.ingredientsTable)
    })
}

function createRequestForButton(b, async) {
    const xhr = new XMLHttpRequest()
    const method = b.getAttribute("data-method")
    const action = b.getAttribute("data-action")
    console.assert(method)
    console.assert(action)
    xhr.open(method, action, async)
    return xhr
}

function parseIngredients(text) {
    const lines = text.split("\n")
    const ingredients = new Array(lines.length)
    let off = 0
    for(let i = 0; i < lines.length; i++) {
        const line = lines[i]
        const arr = line.split(/\s+/, 3)
        if(arr.length === 0)
            continue;

        let ingredient;
        if(arr.length === 1) {
            ingredient = {
                amount: "",
                unit: "",
                name: arr[0],
            }
        } else if(arr.length === 2) {
            ingredient = {
                amount: arr[0],
                unit: "",
                name: arr[1],
            }
        } else {
            ingredient = {
                amount: arr[0],
                unit: arr[1],
                name: arr[2],
            }
        }
        ingredients[off++] = ingredient
    }
    return ingredients.slice(0, off)
}

function initIngredientsImport(importInformation) {
    importInformation.importButton.addEventListener("click", function() {
        const text = importInformation.textArea.value
        const ingredients = parseIngredients(text)

        const targetSection = importInformation.targetSection
        const table = targetSection.ingredientsTable
        for(let i = 0; i < ingredients.length; i++) {
            const ingredient = ingredients[i]
            const row = addRowToTable(table)
            row.ingredient.nameInput.value = ingredient.name
            row.ingredient.unitInput.value = ingredient.unit
            row.ingredient.amountInput.value = ingredient.amount
        }

        importInformation.modal.modal("hide")
    })
}

$(function() {
    const info = createInitialState()
    defaultRow = document.getElementById("default-row").firstChild
    defaultSection = document.getElementById("default-section").firstChild
    console.assert(defaultRow && defaultSection)

    const importInformation = {
        importButton: document.getElementById("import-ingredients-text-button"),
        textArea: document.getElementById("import-ingredients-text-area"),
        modal: $(document.getElementById("import-ingredients-text-modal")),
        targetSection: null,
        reset: function() {
            this.textArea.value = ""
        }
    }
    console.assert(importInformation.importButton && importInformation.textArea && importInformation.modal)

    info.findSections().each(function(i, section) {
        initSection(section, defaultRow, importInformation)
    })

    initIngredientsImport(importInformation)

    const addButton = document.getElementById("button-add-section")
    addButton.addEventListener("click", function() {
        const e = defaultSection.cloneNode(true)
        info.ingredientsSections.appendChild(e)
        initSection(e, defaultRow, importInformation)
    })

    const errorToast = $(document.getElementById("toast-submit-failed"))
    const errorToastContent = errorToast.find(".toast-body")[0]
    const submitButton = document.getElementById("submit-recipe-button")
    const deleteButton = document.getElementById("delete-recipe-button")
    const deleteModalButton = document.getElementById("delete-recipe-modal-button")
    console.assert(submitButton)

    function setButtonsDisabled(disabled) {
        submitButton.disabled = disabled
        if(deleteButton) {
            deleteButton.disabled = disabled
            deleteModalButton.disabled = disabled
        }
    }

    function serverError(xhr) {
        const text = xhr.responseText
        errorToastContent.innerText = "Antwort des Servers: " + text
        errorToast.toast("dispose")
        errorToast.toast("show")
        console.error("Failed with status code " + xhr.status + " (" + xhr.statusText + "): " + text)
        setButtonsDisabled(false)
    }

    function saveRecipe() {
        const title = info.title.value
        const description = info.description.value
        const instructions = info.instructions.value
        const source = info.source.value
        const sections = info.findSections()
        const resArray = new Array(sections.length)
        const image = document.getElementById("image").getAttribute("src")
        for(let i = 0; i < sections.length; i++) {
            const section = sections[i]
            const heading = section.headingInput.value

            const tableRows = section.ingredients()
            const ingredientsArray = new Array(tableRows.length)
            for(let j = 0; j < tableRows.length; j++) {
                const row = tableRows[j]

                ingredientsArray[j] = {
                    Amount: row.ingredient.amountInput.value,
                    Unit: row.ingredient.unitInput.value,
                    Name: row.ingredient.nameInput.value,
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
        const xhr = createRequestForButton(submitButton, true)
        xhr.setRequestHeader("Content-Type", 'application/json; charset=UTF-8')
        xhr.onreadystatechange = XHRResultHandler(xhr, doRedirect, serverError)
        xhr.send(json)
    }

    submitButton.addEventListener("click", function() {
        setButtonsDisabled(true)
        saveRecipe()
    })

    if(deleteButton) {
        deleteButton.addEventListener("click", function() {
            setButtonsDisabled(true)
            const xhr = createRequestForButton(deleteButton, true)
            xhr.onreadystatechange = XHRResultHandler(xhr, doRedirect, serverError)
            xhr.send()
        })
    }
})