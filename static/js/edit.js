function initToolbar(node, toolbar) {
    const deleteButton = toolbar.querySelector(".button-delete")
    const upButton = toolbar.querySelector(".button-up")
    const downButton = toolbar.querySelector(".button-down")
    deleteButton.addEventListener("click", function () {
        removeNode(node)
    })
    upButton.addEventListener("click", function () {
        moveUp(node)
    })
    downButton.addEventListener("click", function () {
        moveDown(node)
    })
}

function extendRow(row) {
    row.ingredient = {
        nameInput: row.querySelector(".ingredient-name-input"),
        unitInput: row.querySelector(".ingredient-unit-input"),
        amountInput: row.querySelector(".ingredient-amount-input"),
    }
}

function initListenersForRow(row) {
    initToolbar(row, row)
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
    extendSection(section)
    const toolbar = section.querySelector(".toolbar-wrapper")
    console.assert(toolbar.length !== 0)
    initToolbar(section, toolbar)
    toolbar.querySelector(".button-import").addEventListener("click", function () {
        importInformation.targetSection = section
        importInformation.reset()
        importInformation.modal.show()
    })

    section.ingredients().forEach(function (row) {
        extendRow(row)
        initListenersForRow(row)
    })
    const addButton = section.querySelector(".button-add")
    addButton.addEventListener("click", function () {
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

function splitAtMostNParts(str, pattern, n) {
    let res = [];
    while (str.length !== 0) {
        if (n === 1)
            break;

        --n;
        let match = str.match(pattern);
        if (match === null)
            break;

        res.push(str.slice(0, match.index));
        str = str.slice(match.index + match[0].length);
    }
    res.push(str);
    return res;
}

function ingredientFromInput(input) {
    const arr = splitAtMostNParts(input, /\s+/, 3);
    if (arr.length === 0 || arr.length === 1 && arr[0] === "")
        return null;
    const hasNumber = arr[0].match(/\d/) != null;
    if (arr.length === 1 || !hasNumber) {
        return {
            amount: "",
            unit: "",
            name: input,
        }
    } else if (arr.length === 2) {
        return {
            amount: arr[0],
            unit: "",
            name: arr[1],
        }
    } else {
        return {
            amount: arr[0],
            unit: arr[1],
            name: arr[2],
        }
    }
}

function parseIngredients(text) {
    const lines = text.split("\n")
    const ingredients = new Array(lines.length)
    let off = 0
    for (let i = 0; i < lines.length; i++) {
        const line = lines[i].trim();
        const ingredient = ingredientFromInput(line);
        if (ingredient === null) {
            continue;
        }
        ingredients[off++] = ingredient;
    }
    return ingredients.slice(0, off)
}

function initIngredientsImport(importInformation) {
    importInformation.importButton.addEventListener("click", function () {
        const text = importInformation.textArea.value
        const ingredients = parseIngredients(text)

        const targetSection = importInformation.targetSection
        const table = targetSection.ingredientsTable
        for (let i = 0; i < ingredients.length; i++) {
            const ingredient = ingredients[i]
            const row = addRowToTable(table)
            row.ingredient.nameInput.value = ingredient.name
            row.ingredient.unitInput.value = ingredient.unit
            row.ingredient.amountInput.value = ingredient.amount
        }

        importInformation.modal.hide()
    })
}

document.addEventListener("DOMContentLoaded", function () {
    const info = createInitialState()
    defaultRow = document.getElementById("default-row").firstChild
    defaultSection = document.getElementById("default-section").firstChild
    console.assert(defaultRow && defaultSection)

    const importInformation = {
        importButton: document.getElementById("import-ingredients-text-button"),
        textArea: document.getElementById("import-ingredients-text-area"),
        modal: new bootstrap.Modal(document.getElementById("import-ingredients-text-modal"), {}),
        targetSection: null,
        reset: function () {
            this.textArea.value = ""
        }
    }
    console.assert(importInformation.importButton && importInformation.textArea && importInformation.modal)

    info.findSections().forEach(function (section) {
        initSection(section, defaultRow, importInformation)
    })

    initIngredientsImport(importInformation)

    const addButton = document.getElementById("button-add-section")
    addButton.addEventListener("click", function () {
        const e = defaultSection.cloneNode(true)
        info.ingredientsSections.appendChild(e)
        initSection(e, defaultRow, importInformation)
    })

    const errorToastElement = document.getElementById("toast-submit-failed");
    const errorToast = new bootstrap.Toast(errorToastElement)
    const errorToastContent = errorToastElement.querySelector(".toast-body")
    const submitButton = document.getElementById("submit-recipe-button")
    const deleteButton = document.getElementById("delete-recipe-button")
    const deleteModalButton = document.getElementById("delete-recipe-modal-button")

    function setButtonsDisabled(disabled) {
        submitButton.disabled = disabled
        if (deleteButton) {
            deleteButton.disabled = disabled
            deleteModalButton.disabled = disabled
        }
    }

    function serverError(xhr) {
        const text = xhr.responseText.trim();
        let message;
        if (errors.hasOwnProperty(text)) {
            message = errors[text];
        } else {
            message = "Antwort des Servers: " + text;
        }
        errorToastContent.innerText = message;
        errorToast.show()
        console.error("Failed with status code " + xhr.status + " (" + xhr.statusText + "): " + text)
        setButtonsDisabled(false)
    }

    function saveRecipe() {
        const title = info.title.value
        const description = info.description.value
        const instructions = info.instructions.value
        const source = info.source.value
        const categories = info.categories.value.split(",")
        const sections = info.findSections()
        const resArray = new Array(sections.length)
        const imageElement = document.getElementById("image")
        const image = imageElement ? imageElement.getAttribute("src") : ""
        for (let i = 0; i < sections.length; i++) {
            const section = sections[i]
            const heading = section.headingInput.value

            const tableRows = section.ingredients()
            const ingredientsArray = new Array(tableRows.length)
            for (let j = 0; j < tableRows.length; j++) {
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
            Categories: categories,
        }
        const json = JSON.stringify(res)
        const xhr = createRequestForButton(submitButton, true)
        xhr.setRequestHeader("Content-Type", 'application/json; charset=UTF-8')
        xhr.onreadystatechange = XHRResultHandler(xhr, doRedirect, serverError)
        xhr.send(json)
    }

    submitButton.addEventListener("click", function () {
        setButtonsDisabled(true)
        saveRecipe()
    })

    if (deleteButton) {
        deleteButton.addEventListener("click", function () {
            setButtonsDisabled(true)
            const xhr = createRequestForButton(deleteButton, true)
            xhr.onreadystatechange = XHRResultHandler(xhr, doRedirect, serverError)
            xhr.send()
        })
    }
})