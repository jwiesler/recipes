function extendRow(row) {
    row.ingredient = {
        amountSpan: row.querySelector(".ingredient-amount-number"),
        unit: row.querySelector(".ingredient-amount-unit").innerText,
        name: row.querySelector(".ingredient-name").innerText,
        parseAmount: function () {
            return parseFloat(this.amountSpan.innerText)
        },
        scaleAmountByFactor: function (factor) {
            this.amountSpan.innerText = Math.round((this.originalAmount * factor + Number.EPSILON) * 1000) / 1000
        },
    }
    row.ingredient.originalAmount = row.ingredient.parseAmount()
}

function scanIngredients(info, validRows) {
    info.findSections().forEach(function (section) {
        extendSection(section)
        section.ingredients().forEach(function (row) {
            extendRow(row)
            const amount = row.ingredient.originalAmount
            const amountValid = !isNaN(amount)
            if (!amountValid)
                return

            validRows.push(row)
        })
    })
}

document.addEventListener("DOMContentLoaded", function () {
    const validRows = []
    const info = createInitialState()

    if (!info.ingredients)
        return

    scanIngredients(info, validRows)

    function scaleAllByFactor(factor) {
        validRows.forEach(function (row) {
            row.ingredient.scaleAmountByFactor(factor)
        })
    }

    const scaleIngredientAmountInput = document.getElementById("scale-ingredient-amount")
    const scaleIngredientButton = document.getElementById("scale-ingredient-button")
    const scaleIngredientSelect = document.getElementById("scale-ingredient-select")

    scaleIngredientButton.addEventListener("click", function () {
        const index = scaleIngredientSelect.selectedIndex
        if (index === -1)
            return
        const amount = scaleIngredientAmountInput.value
        if (!amount || !scaleIngredientAmountInput.validity.valid)
            return

        const option = scaleIngredientSelect.options[index]
        const totalAmount = option.getAttribute("data-total-amount")
        const factor = amount / totalAmount
        if (isNaN(factor))
            return
        scaleAllByFactor(factor)
    });

    let screenLock = null;
    navigator.wakeLock.request('screen').then(lock => {
        screenLock = lock;
    });

    document.addEventListener('visibilitychange', async () => {
        if (screenLock !== null && document.visibilityState === 'visible') {
            screenLock = await navigator.wakeLock.request('screen');
        }
    });
});
