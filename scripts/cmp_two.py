import csv, sys, os

def find_ok_rows(file1, file2):
    file1_data = {}
    
    with open(file1, mode='r', encoding='utf-8') as f1:
        reader1 = csv.reader(f1)
        for row in reader1:
           key = row[0]+'_'+row[1]   #block_tx
           file1_data[key] = row[12] #result

    result = []
    with open(file2, mode='r', encoding='utf-8') as f2:
        reader2 = csv.reader(f2)
        for row in reader2:
           key = row[0]+'_'+row[1] #block_tx
           if key in file1_data:
              file1_row = file1_data[key]
              if ('OK' not in file1_row and 'OK' in row[12]):
                 result.append(key + " " + row[2])
    
    return result

file1 = sys.argv[1] #new
file2 = sys.argv[2] #old
if len(sys.argv) > 3:
   print("swap..", end=' ', flush=True)
   file1, file2 = file2, file1

fn1 = os.path.splitext(os.path.basename(file1))[0]
fn2 = os.path.splitext(os.path.basename(file2))[0]

result = find_ok_rows(file1, file2)

if result:
   print("NOT EMPTY")
   with open(f"{fn1}_{fn2}cmp.txt", 'w') as filename:
      for row in result:
          filename.write(row + "\n")
else:
   print("EMPTY")

