function extendRow(row) {
    const jrow = $(row)
    row.ingredient = {
        amountSpan: jrow.find(".ingredient-amount-number")[0],
        unit: jrow.find(".ingredient-amount-unit")[0].innerText,
        name: jrow.find(".ingredient-name")[0].innerText,
        parseAmount: function() {
            return parseFloat(this.amountSpan.innerText)
        },
        scaleAmountByFactor: function(factor) {
            this.amountSpan.innerText = Math.round((this.originalAmount * factor + Number.EPSILON) * 1000) / 1000
        },
    }
    row.ingredient.originalAmount = row.ingredient.parseAmount()
}

function createIngredientInfo(name, unit) {
    return {
        name: name,
        unit: unit,
        amount: 0,
    }
}

function scanIngredients(info, ingredientInfos, validRows) {
    info.findSections().each(function(i, section) {
        extendSection(section)
        section.ingredients().forEach(function(row) {
            extendRow(row)

            const ingredient = row.ingredient
            const amount = ingredient.parseAmount()
            const amountValid = !isNaN(amount)
            if(!amountValid)
                return

            const key = makeIngredientKey(ingredient)
            let ingredientInfo = ingredientInfos.get(key)
            if(ingredientInfo === undefined) {
                ingredientInfo = createIngredientInfo(ingredient.name, ingredient.unit)
                ingredientInfos.set(key, ingredientInfo)
            }

            ingredientInfo.amount += amount
            validRows.push(row)
        })
    })
}

function makeIngredientKey(ingredient) {
    let text = ingredient.name
    if(ingredient.unit) {
        text += " (" + ingredient.unit + ")"
    }
    return text
}

$(function() {
    const ingredientInfos = new Map()
    const validRows = []
    const info = createInitialState()

    scanIngredients(info, ingredientInfos, validRows)

    function scaleAllByFactor(factor) {
        validRows.forEach(function(row) {
            row.ingredient.scaleAmountByFactor(factor)
        })
    }

    function scaleIngredient(ingredient, amount) {
        const ingredientInfo = ingredientInfos.get(ingredient)
        if(ingredientInfo === undefined)
            return
        const factor = amount / ingredientInfo.amount
        scaleAllByFactor(factor)
    }

    const scaleIngredientAmountInput = $("#scale-ingredient-amount")[0]
    const scaleIngredientButton = $("#scale-ingredient-button")[0]
    const scaleIngredientSelect = $("#scale-ingredient-select")[0]

    scaleIngredientButton.addEventListener("click", function() {
        const name = scaleIngredientSelect.value
        const amount = scaleIngredientAmountInput.value
        if(!name || !amount)
            return
        scaleIngredient(name, amount)
    })

    const ingredients = Array.from(ingredientInfos.keys()).sort()
    ingredients.forEach(function(k) {
        scaleIngredientSelect.appendChild(new Option(k, k))
    })
})