#!/usr/bin/env python3
import re
import sys

# Hardcoding flag to control expression printing
PRINT_EXPRESSION = False # Set to True to print expressions, False to suppress them
FIVE_CATEGORIES = False # Set to True to print statistics in five categories


def analyze_log_file(filename):
    """
    Reads a text file, detects specific patterns, and sums up the numbers
    associated with each pattern category.  Handles potential file errors.

    Args:
        filename (str): The name of the text file to analyze.

    Returns:
        dict: A dictionary containing the sums and expressions for each
              pattern category. Returns an empty dictionary if the file
              does not exist, if there's an error reading the file, or if
              no patterns are found.  The keys are the pattern names
              (e.g., "PASSED", "FAILED"), and the values are dictionaries
              containing the 'sum' (integer) and 'expression' (string) for
              each category.
    """
    results = {}
    try:
        with open(filename, 'r') as file:
            for line in file:
                # Pattern: Word followed by a space and a number.
                # Updated pattern to match only the specified categories
                for match in re.findall(r"(Total|PASSED|FAILED|TACParseError|TACPanic|TACTimeout|IllJumpTx|IllPhiTx|TACThrowErr|NoTacTx|CallTraceErr|GasTraceErr)\s+(\d+)", line):
                    category, number_str = match
                    try:
                        if FIVE_CATEGORIES:
                            if category in ("TACParseError", "NoTacTx"):
                                category = "DecompilerError"
                            if category in ("TACPanic", "IllJumpTx", "IllPhiTx", "TACThrowErr", "CallTraceErr", "GasTraceErr"):
                                category = "RuntimeError"
                        number = int(number_str)
                        if category not in results:
                            results[category] = {'sum': 0, 'expression': []}
                        results[category]['sum'] += number
                        results[category]['expression'].append(number_str)
                    except ValueError:
                        print(f"Warning: Invalid number format for category '{category}': '{number_str}' in file '{filename}'")
                        # Consider logging the error or handling it as needed.
    except FileNotFoundError:
        print(f"Error: File not found: {filename}")
        # It's important to return an empty dict, as the docstring specifies.
        return {}
    except Exception as e:
        print(f"An error occurred while reading the file '{filename}': {e}")
        # Consider logging the error.  Returning {} is consistent with the
        # FileNotFound case and the docstring.
        return {}

    return results


def divide_expressions(expr1, expr2):
    """
    Divides two expressions represented as strings.  Handles empty expressions.

    Args:
        expr1 (str): The first expression string.
        expr2 (str): The second expression string (denominator).

    Returns:
        str: The division of the two expressions, or "N/A" if the denominator is empty.
    """
    if not expr2:
        return "N/A"  # Handle empty denominator
    return f"({expr1}) / ({expr2})"



def main():
    """
    Main function to execute the log file analysis and print the results.
    Gets the filenames from the command line arguments.
    """
    if len(sys.argv) < 2:
        print("Usage: python script.py <file1> <file2> ...")
        print("Please provide one or more input file names as command-line arguments.")
        sys.exit(1)  # Use sys.exit() for a cleaner exit

    all_results = {}
    for filename in sys.argv[1:]:
        results = analyze_log_file(filename)
        if results:  # Only process if analyze_log_file returned something
            print(f"Results for file: {filename}")
            for category, data in results.items():
                total_sum = data['sum']
                if PRINT_EXPRESSION:
                    expression = '+'.join(map(str, data['expression']))
                    print(f"  {category}: {total_sum}  ({expression})")
                else:
                    print(f"  {category}: {total_sum}")
            # Combine results from all files
            for category, data in results.items():
                if category not in all_results:
                    all_results[category] = {'sum': 0, 'expression': []}
                all_results[category]['sum'] += data['sum']
                all_results[category]['expression'].extend(data['expression'])

    if all_results:
        print("\nCombined Results:")
        total_combined_sum = all_results["Total"]["sum"]  # Use Total sum directly.
        total_expression = '+'.join(map(str, all_results["Total"]["expression"])) # Get "Total" expression
        for category, data in all_results.items():
            total_sum = data['sum']
            if PRINT_EXPRESSION:
                expression = '+'.join(map(str, data['expression']))
                divided_expression = divide_expressions(expression, total_expression) # Calculate division
                percentage = (total_sum / total_combined_sum) * 100 if total_combined_sum else 0
                print(f"  {category}: {total_sum}  Percentage: {percentage:.2f}%  Expression: {divided_expression}") # Changed "Division" to "Expression"
            else:
                percentage = (total_sum / total_combined_sum) * 100 if total_combined_sum else 0
                print(f"  {category}: {total_sum}  Percentage: {percentage:.2f}%")
    else:
        print("No valid files provided or no patterns found in the files.")


if __name__ == "__main__":
    main()
