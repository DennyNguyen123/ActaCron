/**
 * @name calculate_discount
 * @description Computes discounted product price
 * @cron 0 9 * * 1-5
 * @mcp true
 * @param {number} price - Original product price
 * @param {number} discount - Percentage discount (0-100)
 */
function main(params) {
    const price = params.price || 0;
    const discount = params.discount || 0;
    const finalPrice = price * (1 - discount / 100);

    console.log(`Calculated discount: ${price} -> ${finalPrice}`);
    storage.set("last_calculation", { price, finalPrice, timestamp: Date.now() });

    return { final_price: finalPrice };
}
