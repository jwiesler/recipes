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

function scanIngredients(info, validRows) {
    info.findSections().each(function(i, section) {
        extendSection(section)
        section.ingredients().forEach(function(row) {
            extendRow(row)
            const amount = row.ingredient.originalAmount
            const amountValid = !isNaN(amount)
            if(!amountValid)
                return

            validRows.push(row)
        })
    })
}

$(function() {
    const validRows = []
    const info = createInitialState()

    if(!info.ingredients)
        return

    scanIngredients(info, validRows)

    function scaleAllByFactor(factor) {
        validRows.forEach(function(row) {
            row.ingredient.scaleAmountByFactor(factor)
        })
    }

    const scaleIngredientAmountInput = document.getElementById("scale-ingredient-amount")
    const scaleIngredientButton = document.getElementById("scale-ingredient-button")
    const scaleIngredientSelect = document.getElementById("scale-ingredient-select")

    scaleIngredientButton.addEventListener("click", function() {
        const index = scaleIngredientSelect.selectedIndex
        if(index === -1)
            return
        if(!scaleIngredientAmountInput.validity.valid)
            return

        const amount = scaleIngredientAmountInput.value
        const option = scaleIngredientSelect.options[index]
        const totalAmount = option.getAttribute("data-total-amount")
        const factor = amount / totalAmount
        if(isNaN(factor))
            return
        scaleAllByFactor(factor)
    })
})