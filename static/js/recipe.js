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
        valid: true,
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
            let ingredientInfo = ingredientInfos.get(ingredient.name)
            if(ingredientInfo === undefined) {
                ingredientInfo = createIngredientInfo(ingredient.name, ingredient.unit)
                ingredientInfos.set(ingredient.name, ingredientInfo)
            }

            const isValidRow = amountValid && ingredient.unit === ingredientInfo.unit
            ingredientInfo.valid = ingredientInfo.valid && isValidRow
            if(isValidRow) {
                ingredientInfo.amount += amount
            }
            if(amountValid) {
                validRows.push(row)
            }
        })
    })

    const invalids = []
    ingredientInfos.forEach(function(value, key) {
        if(!value.valid)
            invalids.push(key)
    })

    invalids.forEach(function(k) {
        ingredientInfos.delete(k)
    })
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

    const ingredients = Array.from(ingredientInfos.values()).sort(function(a, b) {
        if(a.name < b.name)
            return -1
        if(a.name > b.name)
            return 1
        return 0
    })
    ingredients.forEach(function(k) {
        let text = k.name
        if(k.unit) {
            text += " (" + k.unit + ")"
        }
        scaleIngredientSelect.appendChild(new Option(text, k.name))
    })
})